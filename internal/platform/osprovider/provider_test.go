package osprovider

import (
	"errors"
	"testing"
)

func TestProviderForSupportedFamilies(t *testing.T) {
	tests := []struct {
		name    string
		release string
		manager string
	}{
		{"ubuntu", "ID=ubuntu\n", "apt"},
		{"debian", "ID=debian\n", "apt"},
		{"rocky", "ID=rocky\nID_LIKE=\"rhel centos fedora\"\n", "dnf"},
		{"alma", "ID=almalinux\nID_LIKE=rhel\n", "dnf"},
		{"fedora", "ID=fedora\n", "dnf"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider, err := ProviderForOSRelease(test.release)
			if err != nil {
				t.Fatal(err)
			}
			if got := provider.PackageManagerName(); got != test.manager {
				t.Fatalf("manager = %q, want %q", got, test.manager)
			}
		})
	}
}

func TestProviderRejectsUnknownOS(t *testing.T) {
	_, err := ProviderForOSRelease("ID=arch\n")
	if !errors.Is(err, ErrUnsupportedOS) {
		t.Fatalf("expected ErrUnsupportedOS, got %v", err)
	}
}
