package proxy

import (
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"sync"
	"unicode/utf8"

	"github.com/cloudfoundry-incubator/diego-ssh/helpers"
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/pivotal-golang/lager"
	"golang.org/x/crypto/ssh"
)

type Waiter interface {
	Wait() error
}

type TargetConfig struct {
	Address         string `json:"address"`
	HostFingerprint string `json:"host_fingerprint"`
	User            string `json:"user,omitempty"`
	Password        string `json:"password,omitempty"`
	PrivateKey      string `json:"private_key,omitempty"`
}

type LogMessage struct {
	Guid    string `json:"guid"`
	Message string `json:"message"`
	Index   int    `json:"index"`
}

type Proxy struct {
	logger       lager.Logger
	serverConfig *ssh.ServerConfig
}

func New(
	logger lager.Logger,
	serverConfig *ssh.ServerConfig,
) *Proxy {
	return &Proxy{
		logger:       logger,
		serverConfig: serverConfig,
	}
}

func (p *Proxy) HandleConnection(netConn net.Conn) {
	logger := p.logger.Session("handle-connection")
	defer netConn.Close()

	serverConn, serverChannels, serverRequests, err := ssh.NewServerConn(netConn, p.serverConfig)
	if err != nil {
		return
	}
	defer serverConn.Close()

	clientConn, clientChannels, clientRequests, err := NewClientConn(logger, serverConn.Permissions)
	if err != nil {
		return
	}
	defer clientConn.Close()

	emitLogMessage(logger, serverConn.Permissions)

	go ProxyGlobalRequests(logger, clientConn, serverRequests)
	go ProxyGlobalRequests(logger, serverConn, clientRequests)

	go ProxyChannels(logger, clientConn, serverChannels)
	go ProxyChannels(logger, serverConn, clientChannels)

	Wait(logger, serverConn, clientConn)
}

func emitLogMessage(logger lager.Logger, perms *ssh.Permissions) {
	logMessageJson := perms.CriticalOptions["log-message"]
	if logMessageJson == "" {
		return
	}

	logMessage := &LogMessage{}
	err := json.Unmarshal([]byte(logMessageJson), logMessage)
	if err != nil {
		logger.Error("json-unmarshal-failed", err)
		return
	}

	logs.SendAppLog(logMessage.Guid, logMessage.Message, "SSH", strconv.Itoa(logMessage.Index))
}

func ProxyGlobalRequests(logger lager.Logger, conn ssh.Conn, reqs <-chan *ssh.Request) {
	logger = logger.Session("proxy-global-requests")

	logger.Info("started")
	defer logger.Info("completed")

	for req := range reqs {
		logger.Info("request", lager.Data{
			"type":      req.Type,
			"wantReply": req.WantReply,
			"payload":   req.Payload,
		})
		success, reply, err := conn.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			logger.Error("send-request-failed", err)
			continue
		}

		if req.WantReply {
			req.Reply(success, reply)
		}
	}
}

func ProxyChannels(logger lager.Logger, conn ssh.Conn, channels <-chan ssh.NewChannel) {
	logger = logger.Session("proxy-channels")

	logger.Info("started")
	defer logger.Info("completed")
	defer conn.Close()

	for newChannel := range channels {
		logger.Info("new-channel", lager.Data{
			"channelType": newChannel.ChannelType(),
			"extraData":   newChannel.ExtraData(),
		})

		targetChan, targetReqs, err := conn.OpenChannel(newChannel.ChannelType(), newChannel.ExtraData())
		if err != nil {
			logger.Error("failed-to-open-channel", err)
			if openErr, ok := err.(*ssh.OpenChannelError); ok {
				newChannel.Reject(openErr.Reason, openErr.Message)
			} else {
				newChannel.Reject(ssh.ConnectionFailed, err.Error())
			}
			continue
		}

		sourceChan, sourceReqs, err := newChannel.Accept()
		if err != nil {
			targetChan.Close()
			continue
		}

		go func() {
			helpers.Copy(logger.Session("to-target"), nil, targetChan, sourceChan)
			targetChan.CloseWrite()
		}()
		go func() {
			helpers.Copy(logger.Session("to-source"), nil, sourceChan, targetChan)
			sourceChan.CloseWrite()
		}()

		go ProxyRequests(logger, newChannel.ChannelType(), sourceReqs, targetChan)
		go ProxyRequests(logger, newChannel.ChannelType(), targetReqs, sourceChan)
	}
}

func ProxyRequests(logger lager.Logger, channelType string, reqs <-chan *ssh.Request, channel ssh.Channel) {
	logger = logger.Session("proxy-requests", lager.Data{
		"channel-type": channelType,
	})

	logger.Info("started")
	defer logger.Info("completed")
	defer channel.Close()

	for req := range reqs {
		logger.Info("request", lager.Data{
			"type":      req.Type,
			"wantReply": req.WantReply,
			"payload":   req.Payload,
		})
		success, err := channel.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			logger.Error("send-request-failed", err)
			continue
		}

		if req.WantReply {
			req.Reply(success, nil)
		}
	}
}

func Wait(logger lager.Logger, waiters ...Waiter) {
	wg := &sync.WaitGroup{}
	for _, waiter := range waiters {
		wg.Add(1)
		go func(waiter Waiter) {
			waiter.Wait()
			wg.Done()
		}(waiter)
	}
	wg.Wait()
}

func NewClientConn(logger lager.Logger, permissions *ssh.Permissions) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	if permissions == nil || permissions.CriticalOptions == nil {
		err := errors.New("Invalid permissions from authentication")
		logger.Error("permissions-and-critical-options-required", err)
		return nil, nil, nil, err
	}

	targetConfigJson := permissions.CriticalOptions["proxy-target-config"]
	logger = logger.Session("new-client-conn", lager.Data{
		"proxy-target-config": targetConfigJson,
	})

	var targetConfig TargetConfig
	err := json.Unmarshal([]byte(permissions.CriticalOptions["proxy-target-config"]), &targetConfig)
	if err != nil {
		logger.Error("unmarshal-failed", err)
		return nil, nil, nil, err
	}

	nConn, err := net.Dial("tcp", targetConfig.Address)
	if err != nil {
		logger.Error("dial-failed", err)
		return nil, nil, nil, err
	}

	clientConfig := &ssh.ClientConfig{}

	if targetConfig.User != "" {
		clientConfig.User = targetConfig.User
	}

	if targetConfig.PrivateKey != "" {
		key, err := ssh.ParsePrivateKey([]byte(targetConfig.PrivateKey))
		if err != nil {
			logger.Error("parsing-key-failed", err)
			return nil, nil, nil, err
		}
		clientConfig.Auth = append(clientConfig.Auth, ssh.PublicKeys(key))
	}

	if targetConfig.User != "" && targetConfig.Password != "" {
		clientConfig.Auth = append(clientConfig.Auth, ssh.Password(targetConfig.Password))
	}

	if targetConfig.HostFingerprint != "" {
		clientConfig.HostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			expectedFingerprint := targetConfig.HostFingerprint

			var actualFingerprint string
			switch utf8.RuneCountInString(expectedFingerprint) {
			case helpers.MD5_FINGERPRINT_LENGTH:
				actualFingerprint = helpers.MD5Fingerprint(key)
			case helpers.SHA1_FINGERPRINT_LENGTH:
				actualFingerprint = helpers.SHA1Fingerprint(key)
			}

			if expectedFingerprint != actualFingerprint {
				err := errors.New("Host fingerprint mismatch")
				logger.Error("host-key-fingerprint-mismatch", err)
				return err
			}

			return nil
		}
	}

	conn, ch, req, err := ssh.NewClientConn(nConn, targetConfig.Address, clientConfig)
	if err != nil {
		logger.Error("handshake-failed", err)
		return nil, nil, nil, err
	}

	return conn, ch, req, nil
}
