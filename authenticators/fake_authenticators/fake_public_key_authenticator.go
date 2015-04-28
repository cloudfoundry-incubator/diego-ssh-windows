// This file was generated by counterfeiter
package fake_authenticators

import (
	"sync"

	"github.com/cloudfoundry-incubator/diego-ssh/authenticators"
	"golang.org/x/crypto/ssh"
)

type FakePublicKeyAuthenticator struct {
	AuthenticateStub        func(metadata ssh.ConnMetadata, publicKey ssh.PublicKey) (*ssh.Permissions, error)
	authenticateMutex       sync.RWMutex
	authenticateArgsForCall []struct {
		metadata  ssh.ConnMetadata
		publicKey ssh.PublicKey
	}
	authenticateReturns struct {
		result1 *ssh.Permissions
		result2 error
	}
	PublicKeyStub        func() ssh.PublicKey
	publicKeyMutex       sync.RWMutex
	publicKeyArgsForCall []struct{}
	publicKeyReturns     struct {
		result1 ssh.PublicKey
	}
}

func (fake *FakePublicKeyAuthenticator) Authenticate(metadata ssh.ConnMetadata, publicKey ssh.PublicKey) (*ssh.Permissions, error) {
	fake.authenticateMutex.Lock()
	fake.authenticateArgsForCall = append(fake.authenticateArgsForCall, struct {
		metadata  ssh.ConnMetadata
		publicKey ssh.PublicKey
	}{metadata, publicKey})
	fake.authenticateMutex.Unlock()
	if fake.AuthenticateStub != nil {
		return fake.AuthenticateStub(metadata, publicKey)
	} else {
		return fake.authenticateReturns.result1, fake.authenticateReturns.result2
	}
}

func (fake *FakePublicKeyAuthenticator) AuthenticateCallCount() int {
	fake.authenticateMutex.RLock()
	defer fake.authenticateMutex.RUnlock()
	return len(fake.authenticateArgsForCall)
}

func (fake *FakePublicKeyAuthenticator) AuthenticateArgsForCall(i int) (ssh.ConnMetadata, ssh.PublicKey) {
	fake.authenticateMutex.RLock()
	defer fake.authenticateMutex.RUnlock()
	return fake.authenticateArgsForCall[i].metadata, fake.authenticateArgsForCall[i].publicKey
}

func (fake *FakePublicKeyAuthenticator) AuthenticateReturns(result1 *ssh.Permissions, result2 error) {
	fake.AuthenticateStub = nil
	fake.authenticateReturns = struct {
		result1 *ssh.Permissions
		result2 error
	}{result1, result2}
}

func (fake *FakePublicKeyAuthenticator) PublicKey() ssh.PublicKey {
	fake.publicKeyMutex.Lock()
	fake.publicKeyArgsForCall = append(fake.publicKeyArgsForCall, struct{}{})
	fake.publicKeyMutex.Unlock()
	if fake.PublicKeyStub != nil {
		return fake.PublicKeyStub()
	} else {
		return fake.publicKeyReturns.result1
	}
}

func (fake *FakePublicKeyAuthenticator) PublicKeyCallCount() int {
	fake.publicKeyMutex.RLock()
	defer fake.publicKeyMutex.RUnlock()
	return len(fake.publicKeyArgsForCall)
}

func (fake *FakePublicKeyAuthenticator) PublicKeyReturns(result1 ssh.PublicKey) {
	fake.PublicKeyStub = nil
	fake.publicKeyReturns = struct {
		result1 ssh.PublicKey
	}{result1}
}

var _ authenticators.PublicKeyAuthenticator = new(FakePublicKeyAuthenticator)
