package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anrted/opendeploy/internal/platform/config"
)

func TestEnsureAndIssueClientUsePersistentCA(t *testing.T) {
	directory := t.TempDir()
	cfg := &config.Config{Server: config.ServerConfig{
		ControlPlaneServerName: "opendeploy-control-plane",
		ControlPlaneCA:         filepath.Join(directory, "ca.crt"),
		ControlPlaneCAKey:      filepath.Join(directory, "ca.key"),
		TLSCertificate:         filepath.Join(directory, "server.crt"),
		TLSPrivateKey:          filepath.Join(directory, "server.key"),
	}}
	manager := NewManager(cfg)
	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	if err := manager.Ensure(now); err != nil {
		t.Fatal(err)
	}
	caBefore, err := os.ReadFile(cfg.Server.ControlPlaneCA)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Ensure(now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	caAfter, _ := os.ReadFile(cfg.Server.ControlPlaneCA)
	if string(caBefore) != string(caAfter) {
		t.Fatal("Ensure rotated an existing CA")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM, fingerprint, _, err := manager.IssueClient("server-1", "agent.example", &key.PublicKey, now)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(certificatePEM))
	if block == nil || fingerprint == "" {
		t.Fatal("client certificate was not issued")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := manager.ClientCertPool()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := certificate.Verify(x509.VerifyOptions{
		Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Fatalf("client certificate is not signed by persistent CA: %v", err)
	}
}
