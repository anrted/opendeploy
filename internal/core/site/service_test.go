package site

import "testing"

func TestValidateDomain(t *testing.T) {
	t.Parallel()

	valid := []string{"example.com", "app-1.example.com", "localhost"}
	for _, domain := range valid {
		if err := validateDomain(domain); err != nil {
			t.Errorf("validateDomain(%q) returned %v", domain, err)
		}
	}

	invalid := []string{
		"",
		"bad_domain.example",
		"-bad.example",
		"bad-.example",
		"two..dots",
		"site/path",
	}
	for _, domain := range invalid {
		if err := validateDomain(domain); err == nil {
			t.Errorf("validateDomain(%q) succeeded, want error", domain)
		}
	}
}

func TestNormalizeRootPath(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"/var/www/example/public": "/var/www/example/public",
		"/srv/apps/../example":    "/srv/example",
	} {
		got, err := normalizeRootPath(input)
		if err != nil {
			t.Errorf("normalizeRootPath(%q) returned %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("normalizeRootPath(%q) = %q, want %q", input, got, want)
		}
	}

	for _, input := range []string{"", "relative/path", "/etc/nginx", "/var/www", `C:\sites\app`} {
		if _, err := normalizeRootPath(input); err == nil {
			t.Errorf("normalizeRootPath(%q) succeeded, want error", input)
		}
	}
}

func TestValidatePHPVersion(t *testing.T) {
	t.Parallel()

	valid := " 8.3 "
	if err := validatePHPVersion(&valid); err != nil {
		t.Fatalf("validatePHPVersion returned %v", err)
	}
	if valid != "8.3" {
		t.Fatalf("normalized version = %q, want 8.3", valid)
	}

	for _, value := range []string{"8", "8.3.1", "latest", "8.x"} {
		version := value
		if err := validatePHPVersion(&version); err == nil {
			t.Errorf("validatePHPVersion(%q) succeeded, want error", value)
		}
	}
}
