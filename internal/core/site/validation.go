package site

import (
	"fmt"
	"path"
	"strings"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

func validateDomain(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return apperrors.InvalidInput("domain is required")
	}
	if len(domain) > 253 {
		return apperrors.InvalidInput("domain must be ≤ 253 characters")
	}
	if strings.HasSuffix(domain, ".") {
		domain = strings.TrimSuffix(domain, ".")
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 {
			return apperrors.InvalidInput("domain contains an invalid DNS label")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return apperrors.InvalidInput("domain labels cannot start or end with a hyphen")
		}
		for _, char := range label {
			if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
				(char >= '0' && char <= '9') || char == '-') {
				return apperrors.InvalidInput(fmt.Sprintf("domain contains invalid character: %q", char))
			}
		}
	}
	return nil
}

func normalizeRootPath(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", apperrors.InvalidInput("root_path is required")
	}
	if strings.ContainsRune(root, '\\') || !strings.HasPrefix(root, "/") {
		return "", apperrors.InvalidInput("root_path must be an absolute Linux path")
	}
	clean := path.Clean(root)
	if !isSafeLinuxPath(clean) {
		return "", apperrors.InvalidInput("root_path contains unsupported characters")
	}
	for _, prefix := range []string{"/var/www/", "/srv/"} {
		if clean != strings.TrimSuffix(prefix, "/") && strings.HasPrefix(clean+"/", prefix) {
			return clean, nil
		}
	}
	return "", apperrors.InvalidInput("root_path must be located below /var/www or /srv")
}

func validatePHPVersion(version *string) error {
	if version == nil {
		return nil
	}
	value := strings.TrimSpace(*version)
	parts := strings.Split(value, ".")
	if len(parts) != 2 || !allDigits(parts[0]) || !allDigits(parts[1]) {
		return apperrors.InvalidInput("php_version must use major.minor format")
	}
	*version = value
	return nil
}

func validateSSL(enabled bool, cert, key *string) error {
	if !enabled {
		return nil
	}
	if cert == nil || key == nil || strings.TrimSpace(*cert) == "" || strings.TrimSpace(*key) == "" {
		return apperrors.InvalidInput("ssl_cert and ssl_key are required when SSL is enabled")
	}
	for name, value := range map[string]string{"ssl_cert": *cert, "ssl_key": *key} {
		clean, err := normalizeCertificatePath(value)
		if err != nil {
			return apperrors.InvalidInput(name + " must be an absolute path below /etc/letsencrypt or /var/lib/opendeploy")
		}
		if name == "ssl_cert" {
			*cert = clean
		} else {
			*key = clean
		}
	}
	return nil
}

func normalizeCertificatePath(value string) (string, error) {
	if strings.ContainsRune(value, '\\') || !strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("not an absolute Linux path")
	}
	clean := path.Clean(strings.TrimSpace(value))
	if !isSafeLinuxPath(clean) {
		return "", fmt.Errorf("path contains unsupported characters")
	}
	for _, prefix := range []string{"/etc/letsencrypt/", "/var/lib/opendeploy/"} {
		if strings.HasPrefix(clean, prefix) {
			return clean, nil
		}
	}
	return "", fmt.Errorf("path is outside managed certificate roots")
}

func isSafeLinuxPath(value string) bool {
	if value == "" || value[0] != '/' {
		return false
	}
	for _, char := range value {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '/' || char == '.' ||
			char == '_' || char == '-') {
			return false
		}
	}
	return true
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
