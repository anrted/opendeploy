package remote

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"testing"
	"time"
)

func testCSR(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "agent.example"},
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
}

func TestIssueClientCertificateUsesCSRPublicKey(t *testing.T) {
	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	certificatePEM, fingerprint, expires, err := issueClientCertificate("server-1", "agent.example", testCSR(t), now)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(certificatePEM))
	if block == nil {
		t.Fatal("certificate PEM was not generated")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if certificate.Subject.CommonName != "server-1" || len(fingerprint) != 64 {
		t.Fatalf("certificate identity=%q fingerprint=%q", certificate.Subject.CommonName, fingerprint)
	}
	if !expires.Equal(now.Add(90 * 24 * time.Hour)) {
		t.Fatalf("expiry=%s", expires)
	}
}

func TestIssueClientCertificateRejectsInvalidCSR(t *testing.T) {
	if _, _, _, err := issueClientCertificate("server-1", "agent.example", "not a csr", time.Now()); err == nil {
		t.Fatal("invalid CSR was accepted")
	}
}

func TestHeartbeatHealthThresholds(t *testing.T) {
	if got := healthFromHeartbeat(HeartbeatRequest{State: "online", CPUUsage: 25, MemoryUsage: 40, DiskUsage: 50}); got != "healthy" {
		t.Fatalf("healthy heartbeat classified as %q", got)
	}
	if got := healthFromHeartbeat(HeartbeatRequest{State: "online", DiskUsage: 96}); got != "warning" {
		t.Fatalf("high disk heartbeat classified as %q", got)
	}
}
