package nginx

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/anrted/opendeploy/internal/agent/executor"
)

type memoryFiles struct {
	values map[string][]byte
}

func (m *memoryFiles) Read(name string) ([]byte, error) {
	value, ok := m.values[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), value...), nil
}

func (m *memoryFiles) Write(name string, content []byte, _ fs.FileMode) error {
	m.values[name] = append([]byte(nil), content...)
	return nil
}

func (m *memoryFiles) Delete(name string) error {
	if _, ok := m.values[name]; !ok {
		return os.ErrNotExist
	}
	delete(m.values, name)
	return nil
}

type memoryLinks map[string]string

func (m memoryLinks) Exists(name string) (bool, error) {
	_, ok := m[name]
	return ok, nil
}
func (m memoryLinks) Symlink(target, link string) error {
	m[link] = target
	return nil
}
func (m memoryLinks) Remove(name string) error {
	delete(m, name)
	return nil
}

type scriptedRunner struct {
	calls  []string
	failAt int
}

func (r *scriptedRunner) Run(_ context.Context, binary string, args ...string) (*executor.Result, error) {
	r.calls = append(r.calls, strings.Join(append([]string{binary}, args...), " "))
	if r.failAt > 0 && len(r.calls) == r.failAt {
		return &executor.Result{Stderr: "invalid configuration", ExitCode: 1}, errors.New("command failed")
	}
	return &executor.Result{}, nil
}

func newTestManager(runner *scriptedRunner) (*Manager, *memoryFiles, memoryLinks) {
	files := &memoryFiles{values: make(map[string][]byte)}
	links := make(memoryLinks)
	return &Manager{
		files:        files,
		runner:       runner,
		links:        links,
		availableDir: "/available",
		enabledDir:   "/enabled",
	}, files, links
}

func TestApplyUpsertValidatesAndReloads(t *testing.T) {
	runner := &scriptedRunner{}
	manager, files, links := newTestManager(runner)
	site := Site{Domain: "example.com", RootPath: "/var/www/example/public", PHPVersion: "8.3"}

	if err := manager.Apply(context.Background(), ActionUpsert, site); err != nil {
		t.Fatalf("Apply returned %v", err)
	}

	configPath, enabledPath := manager.paths(site.Domain)
	config := string(files.values[configPath])
	for _, expected := range []string{
		"server_name example.com;",
		"root /var/www/example/public;",
		"php8.3-fpm.sock",
	} {
		if !strings.Contains(config, expected) {
			t.Errorf("rendered config does not contain %q:\n%s", expected, config)
		}
	}
	if links[enabledPath] != configPath {
		t.Fatalf("enabled link = %q, want %q", links[enabledPath], configPath)
	}
	if got := strings.Join(runner.calls, ","); got != "nginx -t,nginx -s reload" {
		t.Fatalf("commands = %q", got)
	}
}

func TestApplyRollsBackWhenValidationFails(t *testing.T) {
	runner := &scriptedRunner{failAt: 1}
	manager, files, links := newTestManager(runner)
	site := Site{Domain: "example.com", RootPath: "/var/www/new"}
	configPath, enabledPath := manager.paths(site.Domain)
	files.values[configPath] = []byte("old configuration")

	err := manager.Apply(context.Background(), ActionUpsert, site)
	if err == nil {
		t.Fatal("Apply succeeded, want validation error")
	}
	if got := string(files.values[configPath]); got != "old configuration" {
		t.Fatalf("configuration after rollback = %q", got)
	}
	if _, enabled := links[enabledPath]; enabled {
		t.Fatal("site remained enabled after rollback")
	}
	if got := strings.Join(runner.calls, ","); got != "nginx -t,nginx -t,nginx -s reload" {
		t.Fatalf("rollback commands = %q", got)
	}
}

func TestValidateRejectsTraversalAndUnsafeCertificates(t *testing.T) {
	tests := []Site{
		{Domain: "example.com", RootPath: "/var/www/../../etc"},
		{
			Domain: "example.com", RootPath: "/var/www/example",
			SSLEnabled: true, SSLCert: "/etc/letsencrypt/live/example/fullchain.pem\ninclude /tmp/x",
			SSLKey: "/etc/letsencrypt/live/example/privkey.pem",
		},
	}
	for _, site := range tests {
		if err := validate(ActionUpsert, site); err == nil {
			t.Errorf("validate(%+v) succeeded, want error", site)
		}
	}
}
