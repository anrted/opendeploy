package acme

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/http/webroot"
	"github.com/go-acme/lego/v4/registration"
)

// MyUser implements registration.User
type MyUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *MyUser) GetEmail() string {
	return u.Email
}
func (u MyUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *MyUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

type Manager struct {
	email    string
	basePath string // Directory to store certs and account info
}

func NewManager(email string, basePath string) *Manager {
	return &Manager{
		email:    email,
		basePath: basePath,
	}
}

// ChallengeType defines how domain ownership is verified.
type ChallengeType string

const (
	HTTPChallenge ChallengeType = "http-01"
	DNSChallenge  ChallengeType = "dns-01"
)

// ObtainCertificate obtains a certificate for domains using the specified challenge type.
func (m *Manager) ObtainCertificate(domains []string, challenge ChallengeType, webrootDir string, dnsProvider challenge.Provider) (*certificate.Resource, error) {
	// Create a user. New accounts need an email and private key to start.
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	myUser := MyUser{
		Email: m.email,
		key:   privateKey,
	}

	config := lego.NewConfig(&myUser)

	// This CA URL is configured for a local test server.
	config.CADirURL = lego.LEDirectoryProduction
	config.Certificate.KeyType = certcrypto.RSA2048

	// A client facilitates communication with the CA server.
	client, err := lego.NewClient(config)
	if err != nil {
		return nil, err
	}

	if challenge == HTTPChallenge {
		provider, err := webroot.NewHTTPProvider(webrootDir)
		if err != nil {
			return nil, err
		}
		err = client.Challenge.SetHTTP01Provider(provider)
		if err != nil {
			return nil, err
		}
	} else if challenge == DNSChallenge {
		if dnsProvider == nil {
			return nil, fmt.Errorf("dns provider is required for dns-01 challenge")
		}
		err = client.Challenge.SetDNS01Provider(dnsProvider)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unsupported challenge type: %s", challenge)
	}

	// New users will need to register
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, err
	}
	myUser.Registration = reg

	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return nil, err
	}

	// Save certificates
	certPath := filepath.Join(m.basePath, domains[0])
	if err := os.MkdirAll(certPath, 0755); err != nil {
		return nil, err
	}

	certFile := filepath.Join(certPath, "fullchain.pem")
	keyFile := filepath.Join(certPath, "privkey.pem")

	if err := os.WriteFile(certFile, certificates.Certificate, 0644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyFile, certificates.PrivateKey, 0600); err != nil {
		return nil, err
	}

	return certificates, nil
}
