package backup

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// SystemRuntime quiesces writers before restore and verifies both OpenDeploy
// services after configuration and database files have been replaced.
type SystemRuntime struct{}

func (SystemRuntime) BeforeRestore(ctx context.Context) error {
	for _, unit := range []string{"opendeploy-core.service", "opendeploy-agent.service"} {
		if output, err := exec.CommandContext(ctx, "systemctl", "stop", unit).CombinedOutput(); err != nil { //nolint:gosec // fixed unit allowlist
			return fmt.Errorf("stop %s: %s: %w", unit, strings.TrimSpace(string(output)), err)
		}
	}
	return nil
}

func (SystemRuntime) AfterRestore(ctx context.Context) error {
	if output, err := exec.CommandContext(ctx, "systemctl", "daemon-reload").CombinedOutput(); err != nil { //nolint:gosec // fixed command
		return fmt.Errorf("systemd daemon-reload: %s: %w", strings.TrimSpace(string(output)), err)
	}
	for _, unit := range []string{"opendeploy-agent.service", "opendeploy-core.service"} {
		if output, err := exec.CommandContext(ctx, "systemctl", "restart", unit).CombinedOutput(); err != nil { //nolint:gosec // fixed unit allowlist
			return fmt.Errorf("restart %s: %s: %w", unit, strings.TrimSpace(string(output)), err)
		}
		if output, err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unit).CombinedOutput(); err != nil { //nolint:gosec // fixed unit allowlist
			return fmt.Errorf("health check %s: %s: %w", unit, strings.TrimSpace(string(output)), err)
		}
	}
	return nil
}
