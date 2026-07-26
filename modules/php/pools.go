package php

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// PoolConfig holds the settings for a single PHP-FPM pool.
type PoolConfig struct {
	Name            string
	User            string
	Group           string
	Listen          string
	MaxChildren     int
	StartServers    int
	MinSpareServers int
	MaxSpareServers int
	MaxRequests     int
	RequestSlowlog  string
	Slowlog         string
	ErrorLog        string
	PhpAdminValues  map[string]string
}

// CreatePool generates a new PHP-FPM pool configuration file and restarts the service.
func (m *Module) CreatePool(ctx context.Context, version string, cfg PoolConfig) error {
	if cfg.MaxChildren == 0 {
		cfg.MaxChildren = 5
	}
	if cfg.StartServers == 0 {
		cfg.StartServers = 2
	}
	if cfg.MinSpareServers == 0 {
		cfg.MinSpareServers = 1
	}
	if cfg.MaxSpareServers == 0 {
		cfg.MaxSpareServers = 3
	}
	if cfg.Listen == "" {
		cfg.Listen = fmt.Sprintf("/run/php/php%s-fpm-%s.sock", version, cfg.Name)
	}

	confTpl := `[%s]
user = %s
group = %s
listen = %s
listen.owner = %s
listen.group = %s
pm = dynamic
pm.max_children = %d
pm.start_servers = %d
pm.min_spare_servers = %d
pm.max_spare_servers = %d
pm.max_requests = %d
`
	confStr := fmt.Sprintf(confTpl,
		cfg.Name,
		cfg.User,
		cfg.Group,
		cfg.Listen,
		cfg.User,
		cfg.Group,
		cfg.MaxChildren,
		cfg.StartServers,
		cfg.MinSpareServers,
		cfg.MaxSpareServers,
		cfg.MaxRequests,
	)

	if cfg.RequestSlowlog != "" && cfg.Slowlog != "" {
		confStr += fmt.Sprintf("request_slowlog_timeout = %s\nslowlog = %s\n", cfg.RequestSlowlog, cfg.Slowlog)
	}

	for k, v := range cfg.PhpAdminValues {
		confStr += fmt.Sprintf("php_admin_value[%s] = %s\n", k, v)
	}

	poolDir := fmt.Sprintf("/etc/php/%s/fpm/pool.d", version)
	// Make sure the directory exists
	if err := m.deps.Agent.DirCreate(ctx, poolDir, 0o755); err != nil {
		return fmt.Errorf("failed to create pool directory %s: %w", poolDir, err)
	}

	confPath := filepath.Join(poolDir, fmt.Sprintf("%s.conf", cfg.Name))

	// Write file using Agent
	err := m.deps.Agent.FileWrite(ctx, confPath, []byte(confStr), 0644)
	if err != nil {
		return fmt.Errorf("failed to write PHP pool config: %w", err)
	}

	svc := fmt.Sprintf("php%s-fpm", version)
	err = m.deps.Agent.ServiceRestart(ctx, svc)
	if err != nil {
		return fmt.Errorf("failed to restart PHP-FPM service: %w", err)
	}

	m.logger.Info("PHP pool created and service restarted", "pool", cfg.Name, "version", version)
	return nil
}

// DeletePool removes a PHP-FPM pool configuration file and restarts the service.
func (m *Module) DeletePool(ctx context.Context, version, poolName string) error {
	poolDir := fmt.Sprintf("/etc/php/%s/fpm/pool.d", version)
	confPath := filepath.Join(poolDir, fmt.Sprintf("%s.conf", poolName))

	// Delete file
	if err := m.deps.Agent.FileDelete(ctx, confPath); err != nil {
		if !strings.Contains(err.Error(), "no such file") {
			return fmt.Errorf("failed to delete pool config: %w", err)
		}
	}

	svc := fmt.Sprintf("php%s-fpm", version)
	if err := m.deps.Agent.ServiceRestart(ctx, svc); err != nil {
		return fmt.Errorf("failed to restart php-fpm after deleting pool: %w", err)
	}
	m.logger.Info("PHP pool deleted", "pool", poolName, "version", version)
	return nil
}
