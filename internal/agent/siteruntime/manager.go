package siteruntime

import (
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"os/user"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anrted/opendeploy/internal/agent/executor"
	"github.com/anrted/opendeploy/internal/agent/filesystem"
	"github.com/anrted/opendeploy/internal/agent/systemd"
)

var domainPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

type Manager struct {
	files   *filesystem.Manager
	shell   *executor.Shell
	systemd *systemd.Manager
	client  *http.Client
}

func New(files *filesystem.Manager, shell *executor.Shell, systemdManager *systemd.Manager) *Manager {
	return &Manager{
		files: files, shell: shell, systemd: systemdManager,
		client: &http.Client{
			Timeout: 2 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (m *Manager) PrepareRoot(path string) error {
	if !below(path, "/var/www") {
		return fmt.Errorf("site root must be below /var/www")
	}
	if err := m.files.MkdirAll(path, 0o755); err != nil {
		return err
	}
	account, err := lookupWebAccount()
	if err != nil {
		return err
	}
	uid, uidErr := strconv.Atoi(account.Uid)
	gid, gidErr := strconv.Atoi(account.Gid)
	if uidErr != nil || gidErr != nil {
		return fmt.Errorf("invalid web account identity")
	}
	return m.files.Chown(path, uid, gid)
}

func lookupWebAccount() (*user.User, error) {
	for _, name := range []string{"www-data", "nginx", "apache"} {
		account, err := user.Lookup(name)
		if err == nil {
			return account, nil
		}
	}
	return nil, fmt.Errorf("no supported web account exists")
}

func (m *Manager) SocketExists(path string) (bool, error) {
	if !below(path, "/run/php") {
		return false, fmt.Errorf("PHP socket path must be below /run/php")
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode()&os.ModeSocket != 0, nil
}

func (m *Manager) HTTPProbe(ctx context.Context, domain string) (int, error) {
	if !validDomain(domain) {
		return 0, fmt.Errorf("invalid probe domain")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1/", nil)
	if err != nil {
		return 0, err
	}
	request.Host = domain
	response, err := m.client.Do(request)
	if err != nil {
		return 0, err
	}
	statusCode := response.StatusCode
	if err := response.Body.Close(); err != nil {
		return 0, err
	}
	return statusCode, nil
}

func (m *Manager) ObtainCertificate(ctx context.Context, domain, webroot, email string) error {
	if !validDomain(domain) {
		return fmt.Errorf("invalid certificate domain")
	}
	if !below(webroot, "/var/www") {
		return fmt.Errorf("certificate webroot must be below /var/www")
	}
	if email != "" {
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email {
			return fmt.Errorf("invalid certificate email")
		}
	}
	if err := m.files.MkdirAll(webroot, 0o755); err != nil {
		return err
	}
	args := []string{"certonly", "--webroot", "-w", webroot, "-d", domain, "--agree-tos", "-n"}
	if email == "" {
		args = append(args, "--register-unsafely-without-email")
	} else {
		args = append(args, "-m", email)
	}
	result, err := m.shell.Run(ctx, "certbot", args...)
	if err != nil || result == nil || result.ExitCode != 0 {
		return fmt.Errorf("certbot failed")
	}
	return nil
}

func validDomain(domain string) bool {
	return len(domain) <= 253 && domainPattern.MatchString(domain) && !strings.Contains(domain, "..")
}

func below(candidate, root string) bool {
	clean := path.Clean(candidate)
	return clean != root && strings.HasPrefix(clean, root+"/")
}
