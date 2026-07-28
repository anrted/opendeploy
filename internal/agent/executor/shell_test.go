package executor

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestValidatorAllowsAPTInstalledQuery(t *testing.T) {
	err := NewValidator().Validate("apt", []string{"list", "--installed"})
	if err != nil {
		t.Fatalf("apt installed query rejected: %v", err)
	}
}

func TestValidatorRejectsDangerousOperandsAndBinaries(t *testing.T) {
	tests := []struct {
		name   string
		binary string
		args   []string
	}{
		{"shell", "sh", []string{"-c", "id"}},
		{"path binary", "/bin/systemctl", []string{"status", "nginx"}},
		{"service injection", "systemctl", []string{"restart", "nginx;id"}},
		{"package option injection", "apt-get", []string{"install", "--config-file=/tmp/x"}},
		{"log traversal", "tail", []string{"-n", "10", "/var/log/../etc/passwd"}},
		{"git mutation", "git", []string{"clone", "https://example.invalid/repo"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := NewValidator().Validate(test.binary, test.args); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestLimitedBufferBoundsOutput(t *testing.T) {
	var buffer limitedBuffer
	input := strings.Repeat("x", maxCommandOutput+128)
	if _, err := buffer.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if got := len(buffer.buffer.String()); got != maxCommandOutput {
		t.Fatalf("stored %d bytes, want %d", got, maxCommandOutput)
	}
	if !strings.HasSuffix(buffer.String(), "[output truncated]") {
		t.Fatal("missing truncation marker")
	}
}

func TestCommandTimeoutIsApplied(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bounded, boundedCancel := withCommandTimeout(ctx, "systemctl")
	defer boundedCancel()
	deadline, ok := bounded.Deadline()
	if !ok || time.Until(deadline) > time.Second {
		t.Fatal("caller deadline was not preserved")
	}
}

func TestValidatorAllowsFail2BanVersion(t *testing.T) {
	err := NewValidator().Validate("fail2ban-client", []string{"-V"})
	if err != nil {
		t.Fatalf("fail2ban version query rejected: %v", err)
	}
}

func TestValidatorAllowsFail2BanManualBan(t *testing.T) {
	err := NewValidator().Validate("fail2ban-client", []string{"set", "opendeploy-manual", "banip", "2001:db8::10"})
	if err != nil {
		t.Fatalf("fail2ban manual ban rejected: %v", err)
	}
}

func TestValidatorAllowsBoundedNginxDiagnostics(t *testing.T) {
	validator := NewValidator()
	for _, test := range []struct {
		binary string
		args   []string
	}{
		{"nginx", []string{"-V"}},
		{"ss", []string{"-tuln"}},
		{"curl", []string{"-s", "--max-time", "1", "http://127.0.0.1/nginx_status"}},
		{"openssl", []string{"x509", "-in", "/etc/letsencrypt/live/example.com/fullchain.pem", "-noout", "-issuer", "-dates"}},
		{"certbot", []string{"renew", "--cert-name", "example.com", "--non-interactive"}},
	} {
		if err := validator.Validate(test.binary, test.args); err != nil {
			t.Errorf("%s diagnostics rejected: %v", test.binary, err)
		}
	}
	if err := validator.Validate("curl", []string{"-s", "https://example.com"}); err == nil {
		t.Fatal("external curl target was accepted")
	}
	if err := validator.Validate("openssl", []string{"x509", "-in", "/etc/shadow", "-noout"}); err == nil {
		t.Fatal("certificate path outside managed roots was accepted")
	}
}
