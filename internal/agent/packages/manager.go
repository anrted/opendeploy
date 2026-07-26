// Package packages provides package manager abstraction for the Agent.
// It supports APT (Debian/Ubuntu) and DNF/YUM (RHEL/CentOS/Fedora).
package packages

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/anrted/opendeploy/internal/agent/executor"
)

// Manager defines the interface for system package management.
type Manager interface {
	Install(ctx context.Context, pkg string) (<-chan string, error)
	Remove(ctx context.Context, pkg string) (<-chan string, error)
	Purge(ctx context.Context, pkg string) (<-chan string, error)
	Update(ctx context.Context, pkg string) (<-chan string, error)
	Upgrade(ctx context.Context) (<-chan string, error)
	Search(ctx context.Context, pkg string) (string, error)
	Installed(ctx context.Context, pkg string) (bool, string, error)
	LatestVersion(ctx context.Context, pkg string) (string, error)
}

// Detect returns the appropriate package manager for the current system.
func Detect(shell *executor.Shell, logger *slog.Logger) (Manager, error) {
	if hasExecutable("apt-get") {
		logger.Info("packages: detected APT package manager")
		return NewAPT(shell, logger), nil
	}
	if hasExecutable("dnf") {
		logger.Info("packages: detected DNF package manager")
		return NewDNF(shell, logger), nil
	}
	if hasExecutable("yum") {
		logger.Info("packages: detected YUM package manager")
		return NewYUM(shell, logger), nil
	}
	return nil, fmt.Errorf("packages: no supported package manager found (apt-get, dnf, yum)")
}

func hasExecutable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ─── APT ───────────────────────────────────────────────────────────────────

type aptManager struct {
	shell  *executor.Shell
	logger *slog.Logger
}

// NewAPT creates an APT package manager.
func NewAPT(shell *executor.Shell, logger *slog.Logger) Manager {
	return &aptManager{shell: shell, logger: logger}
}

func (m *aptManager) Install(ctx context.Context, pkg string) (<-chan string, error) {
	ch := make(chan string, 64)
	if err := m.shell.Stream(ctx, ch, "apt-get", "install", "-y", "-q", pkg); err != nil {
		return nil, fmt.Errorf("apt: install %s: %w", pkg, err)
	}
	return ch, nil
}

func (m *aptManager) Remove(ctx context.Context, pkg string) (<-chan string, error) {
	ch := make(chan string, 64)
	if err := m.shell.Stream(ctx, ch, "apt-get", "remove", "-y", pkg); err != nil {
		return nil, fmt.Errorf("apt: remove %s: %w", pkg, err)
	}
	return ch, nil
}

func (m *aptManager) Update(ctx context.Context, pkg string) (<-chan string, error) {
	ch := make(chan string, 64)
	if err := m.shell.Stream(ctx, ch, "apt-get", "install", "-y", "--upgrade", pkg); err != nil {
		return nil, fmt.Errorf("apt: update %s: %w", pkg, err)
	}
	return ch, nil
}

func (m *aptManager) Purge(ctx context.Context, pkg string) (<-chan string, error) {
	ch := make(chan string, 64)
	if err := m.shell.Stream(ctx, ch, "apt-get", "remove", "--purge", "-y", pkg); err != nil {
		return nil, fmt.Errorf("apt: purge %s: %w", pkg, err)
	}
	return ch, nil
}

func (m *aptManager) Upgrade(ctx context.Context) (<-chan string, error) {
	ch := make(chan string, 64)
	if err := m.shell.Stream(ctx, ch, "apt-get", "upgrade", "-y", "-q"); err != nil {
		return nil, fmt.Errorf("apt: upgrade: %w", err)
	}
	return ch, nil
}

func (m *aptManager) Search(ctx context.Context, pkg string) (string, error) {
	result, err := m.shell.Run(ctx, "apt", "search", pkg)
	if err != nil {
		return "", fmt.Errorf("apt: search %s: %w", pkg, err)
	}
	return result.Stdout, nil
}

func (m *aptManager) LatestVersion(ctx context.Context, pkg string) (string, error) {
	result, err := m.shell.Run(ctx, "apt", "show", pkg)
	if err != nil {
		return "", fmt.Errorf("apt: show %s: %w", pkg, err)
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.HasPrefix(line, "Version: ") {
			return strings.TrimPrefix(line, "Version: "), nil
		}
	}
	return "", fmt.Errorf("version not found for %s", pkg)
}

func (m *aptManager) Installed(ctx context.Context, pkg string) (bool, string, error) {
	result, err := m.shell.Run(ctx, "apt", "list", "--installed", pkg)
	if err != nil {
		return false, "", err
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.HasPrefix(line, pkg+"/") || strings.HasPrefix(line, pkg+" ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return true, parts[1], nil
			}
			return true, "", nil
		}
	}
	if pkg == "nodejs" {
		if result, err := m.shell.Run(ctx, "node", "--version"); err == nil {
			return true, strings.TrimSpace(strings.TrimPrefix(result.Stdout, "v")), nil
		}
	}
	return false, "", nil
}

// ─── DNF ───────────────────────────────────────────────────────────────────

type dnfManager struct {
	shell  *executor.Shell
	logger *slog.Logger
	bin    string // "dnf" or "yum"
}

// NewDNF creates a DNF package manager.
func NewDNF(shell *executor.Shell, logger *slog.Logger) Manager {
	return &dnfManager{shell: shell, logger: logger, bin: "dnf"}
}

// NewYUM creates a YUM package manager.
func NewYUM(shell *executor.Shell, logger *slog.Logger) Manager {
	return &dnfManager{shell: shell, logger: logger, bin: "yum"}
}

func (m *dnfManager) Install(ctx context.Context, pkg string) (<-chan string, error) {
	ch := make(chan string, 64)
	if err := m.shell.Stream(ctx, ch, m.bin, "install", "-y", pkg); err != nil {
		return nil, fmt.Errorf("%s: install %s: %w", m.bin, pkg, err)
	}
	return ch, nil
}

func (m *dnfManager) Remove(ctx context.Context, pkg string) (<-chan string, error) {
	ch := make(chan string, 64)
	if err := m.shell.Stream(ctx, ch, m.bin, "remove", "-y", pkg); err != nil {
		return nil, fmt.Errorf("%s: remove %s: %w", m.bin, pkg, err)
	}
	return ch, nil
}

func (m *dnfManager) Update(ctx context.Context, pkg string) (<-chan string, error) {
	ch := make(chan string, 64)
	if err := m.shell.Stream(ctx, ch, m.bin, "update", "-y", pkg); err != nil {
		return nil, fmt.Errorf("%s: update %s: %w", m.bin, pkg, err)
	}
	return ch, nil
}

func (m *dnfManager) Purge(ctx context.Context, pkg string) (<-chan string, error) {
	return m.Remove(ctx, pkg) // dnf doesn't have a direct equivalent to purge configuration
}

func (m *dnfManager) Upgrade(ctx context.Context) (<-chan string, error) {
	ch := make(chan string, 64)
	if err := m.shell.Stream(ctx, ch, m.bin, "upgrade", "-y"); err != nil {
		return nil, fmt.Errorf("%s: upgrade: %w", m.bin, err)
	}
	return ch, nil
}

func (m *dnfManager) Search(ctx context.Context, pkg string) (string, error) {
	result, err := m.shell.Run(ctx, m.bin, "search", pkg)
	if err != nil {
		return "", fmt.Errorf("%s: search %s: %w", m.bin, pkg, err)
	}
	return result.Stdout, nil
}

func (m *dnfManager) LatestVersion(ctx context.Context, pkg string) (string, error) {
	result, err := m.shell.Run(ctx, m.bin, "info", pkg)
	if err != nil {
		return "", fmt.Errorf("%s: info %s: %w", m.bin, pkg, err)
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.HasPrefix(line, "Version") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}
	return "", fmt.Errorf("version not found for %s", pkg)
}

func (m *dnfManager) Installed(ctx context.Context, pkg string) (bool, string, error) {
	result, err := m.shell.Run(ctx, m.bin, "list", "--installed")
	if err != nil {
		return false, "", err
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.HasPrefix(line, pkg+".") || strings.HasPrefix(line, pkg+" ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return true, parts[1], nil
			}
			return true, "", nil
		}
	}
	if pkg == "nodejs" {
		if result, err := m.shell.Run(ctx, "node", "--version"); err == nil {
			return true, strings.TrimSpace(strings.TrimPrefix(result.Stdout, "v")), nil
		}
	}
	return false, "", nil
}
