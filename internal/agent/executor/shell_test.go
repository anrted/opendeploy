package executor

import "testing"

func TestValidatorAllowsAPTInstalledQuery(t *testing.T) {
	err := NewValidator().Validate("apt", []string{"list", "--installed"})
	if err != nil {
		t.Fatalf("apt installed query rejected: %v", err)
	}
}

func TestValidatorAllowsFail2BanVersion(t *testing.T) {
	err := NewValidator().Validate("fail2ban-client", []string{"--version"})
	if err != nil {
		t.Fatalf("fail2ban version query rejected: %v", err)
	}
}
