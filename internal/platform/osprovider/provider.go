package osprovider

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

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

func NewProvider() (Provider, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("%w: read /etc/os-release: %v", ErrUnsupportedOS, err)
	}
	return ProviderForOSRelease(string(data))
}

func ProviderForOSRelease(content string) (Provider, error) {
	values := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	id := strings.ToLower(values["ID"])
	like := strings.Fields(strings.ToLower(values["ID_LIKE"]))
	switch {
	case id == "ubuntu", id == "debian", contains(like, "debian"):
		return &UbuntuProvider{}, nil
	case id == "rhel", id == "centos", id == "rocky", id == "almalinux",
		id == "fedora", contains(like, "rhel"), contains(like, "fedora"):
		return &RHELProvider{}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedOS, id)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
