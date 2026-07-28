package nginx

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anrted/opendeploy/pkg/contract"
)

const (
	nginxMainConfigPath    = "/etc/nginx/nginx.conf"
	nginxManagedConfigPath = "/etc/nginx/conf.d/opendeploy-settings.conf"
)

var (
	nginxSizePattern  = regexp.MustCompile(`^[1-9][0-9]*[kKmMgG]?$`)
	nginxMIMEPattern  = regexp.MustCompile(`^[A-Za-z0-9.+-]+/[A-Za-z0-9.+*-]+(?:\s+[A-Za-z0-9.+-]+/[A-Za-z0-9.+*-]+)*$`)
	nginxLogPath      = regexp.MustCompile(`^/var/log/nginx/[A-Za-z0-9_.-]+\.log$`)
	nginxErrorLogPath = regexp.MustCompile(`^/var/log/nginx/[A-Za-z0-9_.-]+\.log(?:\s+(?:debug|info|notice|warn|error|crit|alert|emerg))?$`)
)

func (m *Module) SettingsSchema() []contract.SettingField {
	values := m.currentSettings()
	return []contract.SettingField{
		{ID: "worker_processes", Type: "string", Label: "Worker Processes", Description: "Worker process count or auto", Value: values["worker_processes"], Category: "Performance", RequiresRestart: true, ValidationRegex: `^(auto|[1-9][0-9]{0,2})$`},
		{ID: "worker_connections", Type: "number", Label: "Worker Connections", Description: "Maximum simultaneous connections per worker", Value: values["worker_connections"], Category: "Performance", RequiresRestart: true, ValidationRegex: `^[1-9][0-9]{1,6}$`},
		{ID: "keepalive_timeout", Type: "number", Label: "Keepalive Timeout", Description: "Keep-alive timeout in seconds", Value: values["keepalive_timeout"], Category: "Performance", ValidationRegex: `^[1-9][0-9]{0,3}$`},
		{ID: "client_max_body_size", Type: "string", Label: "Client Max Body Size", Description: "Maximum request body size, for example 50m", Value: values["client_max_body_size"], Category: "General", ValidationRegex: `^[1-9][0-9]*[kKmMgG]?$`},
		{ID: "sendfile", Type: "boolean", Label: "Enable Sendfile", Description: "Use the kernel sendfile operation", Value: values["sendfile"], Category: "Performance"},
		{ID: "gzip", Type: "boolean", Label: "Enable GZIP", Description: "Compress supported responses", Value: values["gzip"], Category: "Performance"},
		{ID: "gzip_types", Type: "string", Label: "GZIP MIME Types", Description: "Space-separated MIME types", Value: values["gzip_types"], Category: "Performance", Advanced: true},
		{ID: "server_tokens", Type: "boolean", Label: "Server Tokens", Description: "Expose the Nginx version in responses", Value: values["server_tokens"], Category: "Security"},
		{ID: "access_log", Type: "string", Label: "Access Log", Description: "Global access log path or off", Value: values["access_log"], Category: "Logging", Advanced: true},
		{ID: "error_log", Type: "string", Label: "Error Log", Description: "Global error log path and optional level", Value: values["error_log"], Category: "Logging", Advanced: true},
	}
}

func (m *Module) currentSettings() map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	mainConfig, _ := m.deps.Agent.FileRead(ctx, nginxMainConfigPath)
	managedConfig, _ := m.deps.Agent.FileRead(ctx, nginxManagedConfigPath)
	mainText, managedText := string(mainConfig), string(managedConfig)
	if exitCode, stdout, stderr, err := m.deps.Agent.CommandExecute(ctx, "nginx", "-T"); err == nil && exitCode == 0 {
		effectiveConfig := stdout
		if effectiveConfig == "" {
			effectiveConfig = stderr
		}
		if strings.TrimSpace(effectiveConfig) != "" {
			if managedText == "" {
				managedText = effectiveConfig
			}
			if mainText == "" {
				mainText = effectiveConfig
			}
		}
	}
	return map[string]any{
		"worker_processes":     firstNonEmpty(nginxDirectiveAtDepth(mainText, "worker_processes", 0), "auto"),
		"worker_connections":   firstNonEmpty(nginxDirectiveInBlock(mainText, "events", "worker_connections"), "1024"),
		"keepalive_timeout":    firstNonEmpty(nginxDirective(managedText, "keepalive_timeout"), "65"),
		"client_max_body_size": firstNonEmpty(nginxDirective(managedText, "client_max_body_size"), "50m"),
		"sendfile":             directiveEnabled(managedText, "sendfile", true),
		"gzip":                 directiveEnabled(managedText, "gzip", true),
		"gzip_types":           firstNonEmpty(nginxDirective(managedText, "gzip_types"), "text/plain text/css application/json application/javascript application/xml"),
		"server_tokens":        directiveEnabled(managedText, "server_tokens", false),
		"access_log":           firstNonEmpty(nginxDirective(managedText, "access_log"), "/var/log/nginx/access.log"),
		"error_log":            firstNonEmpty(nginxDirective(managedText, "error_log"), "/var/log/nginx/error.log warn"),
	}
}

func (m *Module) SaveSettings(ctx context.Context, settings map[string]any) error {
	normalized, err := validateNginxSettings(settings)
	if err != nil {
		return err
	}
	mainSnapshot := m.readSnapshot(ctx, nginxMainConfigPath)
	managedSnapshot := m.readSnapshot(ctx, nginxManagedConfigPath)
	if !mainSnapshot.exists {
		return fmt.Errorf("nginx main configuration is unavailable")
	}

	mainConfig, err := replaceDirectiveAtDepth(string(mainSnapshot.content), "worker_processes", normalized["worker_processes"].(string), 0)
	if err != nil {
		return err
	}
	mainConfig, err = replaceDirectiveInBlock(mainConfig, "events", "worker_connections", normalized["worker_connections"].(string))
	if err != nil {
		return err
	}
	managedConfig := renderManagedSettings(normalized)

	if err := m.deps.Agent.FileWrite(ctx, nginxMainConfigPath, []byte(mainConfig), 0o644); err != nil {
		return fmt.Errorf("write nginx.conf: %w", err)
	}
	if err := m.deps.Agent.FileWrite(ctx, nginxManagedConfigPath, []byte(managedConfig), 0o644); err != nil {
		m.restoreSnapshot(ctx, nginxMainConfigPath, mainSnapshot)
		return fmt.Errorf("write managed nginx settings: %w", err)
	}
	if err := m.validateAndReload(ctx); err != nil {
		m.restoreSnapshot(ctx, nginxMainConfigPath, mainSnapshot)
		m.restoreSnapshot(ctx, nginxManagedConfigPath, managedSnapshot)
		_ = m.deps.Agent.ServiceReload(ctx, "nginx")
		return err
	}
	return nil
}

func validateNginxSettings(settings map[string]any) (map[string]any, error) {
	requiredStrings := []string{"worker_processes", "worker_connections", "keepalive_timeout", "client_max_body_size", "gzip_types", "access_log", "error_log"}
	result := make(map[string]any, len(settings))
	for _, key := range requiredStrings {
		value, ok := settings[key]
		if !ok {
			return nil, fmt.Errorf("%s is required", key)
		}
		result[key] = strings.TrimSpace(fmt.Sprint(value))
	}
	for _, key := range []string{"sendfile", "gzip", "server_tokens"} {
		value, ok := settings[key].(bool)
		if !ok {
			return nil, fmt.Errorf("%s must be boolean", key)
		}
		result[key] = value
	}
	if value := result["worker_processes"].(string); value != "auto" {
		if err := boundedInteger(value, 1, 128); err != nil {
			return nil, fmt.Errorf("worker_processes: %w", err)
		}
	}
	if err := boundedInteger(result["worker_connections"].(string), 16, 1_048_576); err != nil {
		return nil, fmt.Errorf("worker_connections: %w", err)
	}
	if err := boundedInteger(result["keepalive_timeout"].(string), 1, 3600); err != nil {
		return nil, fmt.Errorf("keepalive_timeout: %w", err)
	}
	if !nginxSizePattern.MatchString(result["client_max_body_size"].(string)) {
		return nil, fmt.Errorf("client_max_body_size has invalid format")
	}
	if !nginxMIMEPattern.MatchString(result["gzip_types"].(string)) {
		return nil, fmt.Errorf("gzip_types contains invalid MIME types")
	}
	accessLog := result["access_log"].(string)
	if accessLog != "off" && !nginxLogPath.MatchString(accessLog) {
		return nil, fmt.Errorf("access_log must be off or a file below /var/log/nginx")
	}
	if !nginxErrorLogPath.MatchString(result["error_log"].(string)) {
		return nil, fmt.Errorf("error_log must be a file below /var/log/nginx with an optional level")
	}
	return result, nil
}

func boundedInteger(value string, minimum, maximum int) error {
	number, err := strconv.Atoi(value)
	if err != nil || number < minimum || number > maximum {
		return fmt.Errorf("must be between %d and %d", minimum, maximum)
	}
	return nil
}

func renderManagedSettings(settings map[string]any) string {
	onOff := func(key string) string {
		if settings[key].(bool) {
			return "on"
		}
		return "off"
	}
	return fmt.Sprintf(`# Managed by OpenDeploy. Manual changes will be overwritten.
keepalive_timeout %s;
client_max_body_size %s;
sendfile %s;
gzip %s;
gzip_types %s;
server_tokens %s;
access_log %s;
error_log %s;
`, settings["keepalive_timeout"], settings["client_max_body_size"], onOff("sendfile"),
		onOff("gzip"), settings["gzip_types"], onOff("server_tokens"),
		settings["access_log"], settings["error_log"])
}

func directiveEnabled(content, directive string, defaultValue bool) bool {
	switch nginxDirective(content, directive) {
	case "on":
		return true
	case "off":
		return false
	default:
		return defaultValue
	}
}

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func nginxDirectiveAtDepth(content, directive string, targetDepth int) string {
	depth := 0
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.SplitN(rawLine, "#", 2)[0])
		if depth == targetDepth && strings.HasPrefix(line, directive+" ") {
			return strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, directive)), ";")
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
	}
	return ""
}

func nginxDirectiveInBlock(content, block, directive string) string {
	lines := strings.Split(content, "\n")
	start, end, baseDepth := findBlock(lines, block)
	if start < 0 {
		return ""
	}
	return nginxDirectiveAtDepth(strings.Join(lines[start+1:end], "\n"), directive, baseDepth)
}

func replaceDirectiveAtDepth(content, directive, value string, targetDepth int) (string, error) {
	lines := strings.Split(content, "\n")
	depth := 0
	for index, rawLine := range lines {
		line := strings.TrimSpace(strings.SplitN(rawLine, "#", 2)[0])
		if depth == targetDepth && strings.HasPrefix(line, directive+" ") {
			indent := rawLine[:len(rawLine)-len(strings.TrimLeft(rawLine, " \t"))]
			lines[index] = indent + directive + " " + value + ";"
			return strings.Join(lines, "\n"), nil
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
	}
	return "", fmt.Errorf("nginx directive %s was not found", directive)
}

func replaceDirectiveInBlock(content, block, directive, value string) (string, error) {
	lines := strings.Split(content, "\n")
	start, end, _ := findBlock(lines, block)
	if start < 0 {
		return "", fmt.Errorf("nginx block %s was not found", block)
	}
	for index := start + 1; index < end; index++ {
		line := strings.TrimSpace(strings.SplitN(lines[index], "#", 2)[0])
		if strings.HasPrefix(line, directive+" ") {
			indent := lines[index][:len(lines[index])-len(strings.TrimLeft(lines[index], " \t"))]
			lines[index] = indent + directive + " " + value + ";"
			return strings.Join(lines, "\n"), nil
		}
	}
	indent := "    "
	lines = append(lines[:start+1], append([]string{indent + directive + " " + value + ";"}, lines[start+1:]...)...)
	return strings.Join(lines, "\n"), nil
}

func findBlock(lines []string, block string) (int, int, int) {
	depth := 0
	for index, rawLine := range lines {
		line := strings.TrimSpace(strings.SplitN(rawLine, "#", 2)[0])
		if strings.HasPrefix(line, block) && strings.Contains(line, "{") {
			blockDepth := depth + strings.Count(line, "{") - strings.Count(line, "}")
			currentDepth := blockDepth
			for end := index + 1; end < len(lines); end++ {
				clean := strings.TrimSpace(strings.SplitN(lines[end], "#", 2)[0])
				currentDepth += strings.Count(clean, "{") - strings.Count(clean, "}")
				if currentDepth < blockDepth {
					return index, end, 0
				}
			}
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
	}
	return -1, -1, 0
}

var _ contract.SettingsProvider = (*Module)(nil)
