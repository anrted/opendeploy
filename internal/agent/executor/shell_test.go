package executor

import "testing"

func TestValidatorAllowsAPTInstalledQuery(t *testing.T) {
	err := NewValidator().Validate("apt", []string{"list", "--installed"})
	if err != nil {
		t.Fatalf("apt installed query rejected: %v", err)
	}
}
