package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anrted/opendeploy/internal/platform/config"
)

const (
	caLifetime     = 10 * 365 * 24 * time.Hour
	serverLifetime = 825 * 24 * time.Hour
	clientLifetime = 90 * 24 * time.Hour
)

type Manager struct {
	cfg config.ServerConfig
	mu  sync.Mutex
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg.Server}
}

// Ensure creates a persistent private CA and control-plane server identity
// atomically when they do not exist. Operators never need to invoke openssl.
func (m *Manager) Ensure(now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if allExist(m.cfg.ControlPlaneCA, m.cfg.ControlPlaneCAKey, m.cfg.TLSCertificate, m.cfg.TLSPrivateKey) {
		return m.validate()
	}
	for _, path := range []string{m.cfg.ControlPlaneCA, m.cfg.ControlPlaneCAKey, m.cfg.TLSCertificate, m.cfg.TLSPrivateKey} {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("create PKI directory: %w", err)
		}
	}

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	caSerial, err := randomSerial()
	if err != nil {
		return err
	}
	caTemplate := &x509.Certificate{
		SerialNumber: caSerial, Subject: pkix.Name{
			CommonName:   "OpenDeploy Control Plane Root CA",
			Organization: []string{"OpenDeploy"},
		},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(caLifetime),
		IsCA: true, BasicConstraintsValid: true, MaxPathLen: 0,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	caCertificate, err := x509.ParseCertificate(caDER)
	if err != nil {
		return err
	}

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serverSerial, err := randomSerial()
	if err != nil {
		return err
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial, Subject: pkix.Name{
			CommonName:   m.cfg.ControlPlaneServerName,
			Organization: []string{"OpenDeploy"},
		},
		DNSNames:  []string{m.cfg.ControlPlaneServerName},
		NotBefore: now.Add(-time.Minute), NotAfter: minTime(now.Add(serverLifetime), caCertificate.NotAfter),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCertificate, &serverKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	if err := writePEM(m.cfg.ControlPlaneCA, "CERTIFICATE", caDER, 0o644); err != nil {
		return err
	}
	caKeyDER, err := x509.MarshalPKCS8PrivateKey(caKey)
	if err != nil {
		return err
	}
	if err := writePEM(m.cfg.ControlPlaneCAKey, "PRIVATE KEY", caKeyDER, 0o600); err != nil {
		return err
	}
	if err := writePEM(m.cfg.TLSCertificate, "CERTIFICATE", serverDER, 0o644); err != nil {
		return err
	}
	serverKeyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	if err != nil {
		return err
	}
	if err := writePEM(m.cfg.TLSPrivateKey, "PRIVATE KEY", serverKeyDER, 0o600); err != nil {
		return err
	}
	return m.validate()
}

func (m *Manager) IssueClient(serverID, hostname string, publicKey any, now time.Time) (string, string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ca, key, err := m.loadCA()
	if err != nil {
		return "", "", time.Time{}, err
	}
	expires := minTime(now.Add(clientLifetime), ca.NotAfter)
	clientSerial, err := randomSerial()
	if err != nil {
		return "", "", time.Time{}, err
	}
	template := &x509.Certificate{
		SerialNumber: clientSerial, Subject: pkix.Name{
			CommonName: serverID, Organization: []string{"OpenDeploy Agents"},
		},
		DNSNames: []string{hostname}, NotBefore: now.Add(-time.Minute), NotAfter: expires,
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, publicKey, key)
	if err != nil {
		return "", "", time.Time{}, err
	}
	sum := sha256.Sum256(der)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		hex.EncodeToString(sum[:]), expires, nil
}

func (m *Manager) CAPEM() ([]byte, error) {
	return os.ReadFile(m.cfg.ControlPlaneCA)
}

func (m *Manager) ClientCertPool() (*x509.CertPool, error) {
	caPEM, err := m.CAPEM()
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("control-plane CA contains no certificates")
	}
	return pool, nil
}

func (m *Manager) validate() error {
	if _, _, err := m.loadCA(); err != nil {
		return err
	}
	certificatePEM, err := os.ReadFile(m.cfg.TLSCertificate)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(certificatePEM)
	if block == nil {
		return fmt.Errorf("invalid control-plane certificate PEM")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	if err := certificate.VerifyHostname(m.cfg.ControlPlaneServerName); err != nil {
		return fmt.Errorf("control-plane certificate SAN: %w", err)
	}
	return nil
}

func (m *Manager) loadCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certificatePEM, err := os.ReadFile(m.cfg.ControlPlaneCA)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(m.cfg.ControlPlaneCAKey)
	if err != nil {
		return nil, nil, err
	}
	certificateBlock, _ := pem.Decode(certificatePEM)
	keyBlock, _ := pem.Decode(keyPEM)
	if certificateBlock == nil || keyBlock == nil {
		return nil, nil, fmt.Errorf("invalid control-plane CA PEM")
	}
	certificate, err := x509.ParseCertificate(certificateBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	rawKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	key, ok := rawKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("control-plane CA key is not ECDSA")
	}
	return certificate, key, nil
}

func writePEM(path, blockType string, data []byte, mode os.FileMode) error {
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if err := pem.Encode(file, &pem.Block{Type: blockType, Bytes: data}); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}

func allExist(paths ...string) bool {
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	return serial, nil
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}
