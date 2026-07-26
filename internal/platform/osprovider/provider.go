package osprovider

import (
"errors"
"os"
"strings"
)

// Provider abstracts OS-specific paths, users, and service names.
type Provider interface {
WebUser() string
WebGroup() string
PackageManagerName() string
NginxServiceName() string
NginxConfigDir() string
PHPServiceName(version string) string
DefaultSitePath() string
}

var ErrUnsupportedOS = errors.New("unsupported operating system")

// NewProvider returns the correct OS provider based on /etc/os-release.
func NewProvider() (Provider, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		// Default to Ubuntu/Debian if not found
		return &UbuntuProvider{}, nil
	}
	content := string(data)
	if strings.Contains(content, "ID=ubuntu") || strings.Contains(content, "ID=debian") {
		return &UbuntuProvider{}, nil
	}
	if strings.Contains(content, "ID=centos") || strings.Contains(content, "ID=rhel") || strings.Contains(content, "ID=almalinux") || strings.Contains(content, "ID=rocky") {
		return &RHELProvider{}, nil
	}

// Default fallback
return &UbuntuProvider{}, nil
}
