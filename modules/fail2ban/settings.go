package fail2ban

import (
	"context"
	"fmt"
)

func (m *Module) SaveSettings(ctx context.Context, settings map[string]any) error {
	// For now, we simulate saving to fail2ban.local and jail.local
	// In a real implementation, this would parse the maps, write INI files,
	// and run fail2ban-server -t

	// 1. Generate config file content
	
	// 2. Write to temp file
	
	// 3. Test config: fail2ban-server -t -c <temp_dir>
	_, _, _, err := m.agent.CommandExecute(ctx, "fail2ban-server", "-t")
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	
	// 4. Apply config (move temp to real)
	
	// 5. Reload or Restart
	// If any setting had RequiresRestart = true and was changed, restart.
	// Otherwise reload.
	_, _, _, err = m.agent.CommandExecute(ctx, "fail2ban-client", "reload")
	return err
}
