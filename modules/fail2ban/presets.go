package fail2ban

import (
	"context"
	"fmt"
)

type protectionPreset struct {
	filterPath    string
	filterContent string
	jailPath      string
	jailContent   string
}

var protectionPresets = map[string]protectionPreset{
	"sshd": {
		jailPath: "/etc/fail2ban/jail.d/opendeploy-sshd.local",
		jailContent: `[opendeploy-sshd]
enabled = true
filter = sshd
backend = systemd
port = ssh
maxretry = 5
findtime = 10m
bantime = 24h
usedns = no
`,
	},
	"nginx_scanners": {
		filterPath: "/etc/fail2ban/filter.d/opendeploy-nginx-scanners.conf",
		filterContent: `[Definition]
failregex = ^.*directory index of ".*" is forbidden, client: <HOST>, server: .*request: "(?:GET|HEAD|POST) .* HTTP/.*".*$
            ^.*client intended to send too large body: \d+ bytes, client: <HOST>, server: .*request: "POST .* HTTP/.*".*$
ignoreregex =
`,
		jailPath: "/etc/fail2ban/jail.d/opendeploy-nginx-scanners.local",
		jailContent: `[opendeploy-nginx-scanners]
enabled = true
filter = opendeploy-nginx-scanners
port = http,https
logpath = /var/log/nginx/error.log
backend = auto
maxretry = 5
findtime = 10m
bantime = 24h
usedns = no
`,
	},
	"nginx_auth": {
		jailPath: "/etc/fail2ban/jail.d/opendeploy-nginx-auth.local",
		jailContent: `[opendeploy-nginx-auth]
enabled = true
filter = nginx-http-auth
port = http,https
logpath = /var/log/nginx/error.log
backend = auto
maxretry = 5
findtime = 10m
bantime = 24h
usedns = no
`,
	},
	"php_probes": {
		filterPath: "/etc/fail2ban/filter.d/opendeploy-php-probes.conf",
		filterContent: `[Definition]
failregex = ^<HOST> \S+ \S+ \[.*\] "(?:GET|HEAD|POST) [^"]*(?:/\.env|/wp-login\.php|/xmlrpc\.php|/(?:phpmyadmin|pma)(?:/|\s)|phpinfo(?:\.php|=1)|/vendor/phpunit|/cgi-bin/)[^"]* HTTP/.*" \d{3} .*$
ignoreregex =
`,
		jailPath: "/etc/fail2ban/jail.d/opendeploy-php-probes.local",
		jailContent: `[opendeploy-php-probes]
enabled = true
filter = opendeploy-php-probes
port = http,https
logpath = /var/log/nginx/access.log
backend = auto
maxretry = 5
findtime = 10m
bantime = 24h
usedns = no
`,
	},
}

type configSnapshot struct {
	path    string
	content []byte
	existed bool
}

func (m *Module) enableProtectionPreset(ctx context.Context, presetID string) error {
	preset, ok := protectionPresets[presetID]
	if !ok {
		return fmt.Errorf("unknown protection preset: %s", presetID)
	}

	filterSnapshot := m.snapshotConfig(ctx, preset.filterPath)
	jailSnapshot := m.snapshotConfig(ctx, preset.jailPath)

	if preset.filterPath != "" {
		if err := m.agent.FileWrite(ctx, preset.filterPath, []byte(preset.filterContent), 0o644); err != nil {
			return fmt.Errorf("write %s filter: %w", presetID, err)
		}
	}
	if err := m.agent.FileWrite(ctx, preset.jailPath, []byte(preset.jailContent), 0o644); err != nil {
		m.restoreConfig(ctx, filterSnapshot)
		return fmt.Errorf("write %s jail: %w", presetID, err)
	}

	if err := m.agent.ServiceRestart(ctx, "fail2ban"); err != nil {
		m.restoreConfig(ctx, filterSnapshot)
		m.restoreConfig(ctx, jailSnapshot)
		_ = m.agent.ServiceRestart(ctx, "fail2ban")
		return fmt.Errorf("apply %s protection: %w", presetID, err)
	}
	if err := m.agent.ServiceEnable(ctx, "fail2ban"); err != nil {
		return fmt.Errorf("enable fail2ban at boot: %w", err)
	}
	return nil
}

func (m *Module) disableProtectionPreset(ctx context.Context, presetID string) error {
	preset, ok := protectionPresets[presetID]
	if !ok {
		return fmt.Errorf("unknown protection preset: %s", presetID)
	}

	jailSnapshot := m.snapshotConfig(ctx, preset.jailPath)
	if err := m.agent.FileDelete(ctx, preset.jailPath); err != nil {
		return fmt.Errorf("remove %s jail: %w", presetID, err)
	}
	if err := m.agent.ServiceRestart(ctx, "fail2ban"); err != nil {
		m.restoreConfig(ctx, jailSnapshot)
		_ = m.agent.ServiceRestart(ctx, "fail2ban")
		return fmt.Errorf("disable %s protection: %w", presetID, err)
	}
	return nil
}

func (m *Module) snapshotConfig(ctx context.Context, path string) configSnapshot {
	if path == "" {
		return configSnapshot{}
	}
	content, err := m.agent.FileRead(ctx, path)
	return configSnapshot{path: path, content: content, existed: err == nil}
}

func (m *Module) restoreConfig(ctx context.Context, snapshot configSnapshot) {
	if snapshot.path == "" {
		return
	}
	if snapshot.existed {
		_ = m.agent.FileWrite(ctx, snapshot.path, snapshot.content, 0o644)
		return
	}
	_ = m.agent.FileDelete(ctx, snapshot.path)
}
