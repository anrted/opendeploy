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
	"manual": {
		filterPath: "/etc/fail2ban/filter.d/opendeploy-manual.conf",
		filterContent: `[Definition]
failregex = ^<HOST> OPENDEPLOY_MANUAL_BAN$
ignoreregex =
`,
		jailPath: "/etc/fail2ban/jail.d/opendeploy-manual.local",
		jailContent: `[opendeploy-manual]
enabled = true
filter = opendeploy-manual
banaction = %(banaction_allports)s
logpath = /var/log/fail2ban.log
backend = polling
maxretry = 1
findtime = 10m
bantime = -1
usedns = no
`,
	},
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
failregex = ^<HOST> \S+ \S+ \[[^]]+\] "(?:GET|HEAD|POST) [^"]*(?:/wp-login\.php|/wp-admin(?:[/?\s]|$)|/xmlrpc\.php|/\.env(?:[/?\s]|$)|/\.git(?:[/?\s]|$)|/vendor(?:[/?\s]|$)|/composer\.(?:json|lock)|/phpinfo\.php|/cgi-bin(?:[/?\s]|$)|/admin\.php|(?:\.\./|%2e%2e(?:%2f|/)))[^"]* HTTP/[^"]+" (?:400|401|403|404) .*$
ignoreregex =
`,
		jailPath: "/etc/fail2ban/jail.d/opendeploy-nginx-scanners.local",
		jailContent: `[opendeploy-nginx-scanners]
enabled = true
filter = opendeploy-nginx-scanners
port = http,https
logpath = /var/log/nginx/access.log
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
	"nginx_bad_bots": {
		filterPath: "/etc/fail2ban/filter.d/opendeploy-nginx-bad-bots.conf",
		filterContent: `[Definition]
failregex = ^<HOST> \S+ \S+ \[[^]]+\] "[^"]+" \d{3} \S+ "[^"]*" "[^"]*(?i:foda-scanner|masscan|zgrab|sqlmap|nikto)[^"]*"$
ignoreregex =
`,
		jailPath: "/etc/fail2ban/jail.d/opendeploy-nginx-bad-bots.local",
		jailContent: `[opendeploy-nginx-bad-bots]
enabled = true
filter = opendeploy-nginx-bad-bots
port = http,https
logpath = /var/log/nginx/access.log
backend = auto
maxretry = 1
findtime = 10m
bantime = 24h
usedns = no
`,
	},
	"php_probes": {
		filterPath: "/etc/fail2ban/filter.d/opendeploy-php-probes.conf",
		filterContent: `[Definition]
failregex = ^<HOST> \S+ \S+ \[[^]]+\] "(?:GET|HEAD|POST) [^"]*(?:/\.env(?:[/?\s]|$)|/wp-(?:admin|content|includes)(?:[/?\s]|$)|/wp-login\.php(?:[?\s]|$)|/xmlrpc\.php(?:[?\s]|$)|/(?:phpmyadmin|pma)(?:[/?\s]|$)|/phpinfo(?:\.php|=1)|/vendor/phpunit(?:[/?\s]|$)|/cgi-bin(?:[/?\s]|$)|/(?:eval|shell|upload|installer)\.php(?:[?\s]|$)|/artisan(?:[/?\s]|$)|/composer\.lock(?:[?\s]|$)|/database\.sql(?:[?\s]|$)|/(?:backup|site|www)\.zip(?:[?\s]|$))[^"]* HTTP/[^"]+" (?:400|401|403|404) .*$
            ^<HOST> \S+ \S+ \[[^]]+\] "(?:GET|HEAD|POST) /+[^"]*\.php(?:[/?][^"]*)? HTTP/[^"]+" 404 .*$
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

func presetNeedsUpdate(ctx context.Context, agent interface {
	FileRead(context.Context, string) ([]byte, error)
}, preset protectionPreset) bool {
	jail, err := agent.FileRead(ctx, preset.jailPath)
	if err != nil || string(jail) != preset.jailContent {
		return true
	}
	if preset.filterPath == "" {
		return false
	}
	filter, err := agent.FileRead(ctx, preset.filterPath)
	return err != nil || string(filter) != preset.filterContent
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
	if presetID != "manual" {
		return m.SetProtectionPresetEnabled(ctx, presetID, true)
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
		m.restoreConfig(ctx, filterSnapshot)
		m.restoreConfig(ctx, jailSnapshot)
		_ = m.agent.ServiceRestart(ctx, "fail2ban")
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
	disabledSnapshot := m.snapshotConfig(ctx, preset.jailPath+".disabled")
	if presetID != "manual" && jailSnapshot.existed {
		if err := m.agent.FileWrite(ctx, preset.jailPath+".disabled", jailSnapshot.content, 0o644); err != nil {
			return fmt.Errorf("preserve %s settings: %w", presetID, err)
		}
	}
	if err := m.agent.FileDelete(ctx, preset.jailPath); err != nil {
		m.restoreConfig(ctx, disabledSnapshot)
		return fmt.Errorf("remove %s jail: %w", presetID, err)
	}
	if err := m.agent.ServiceRestart(ctx, "fail2ban"); err != nil {
		m.restoreConfig(ctx, jailSnapshot)
		m.restoreConfig(ctx, disabledSnapshot)
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
