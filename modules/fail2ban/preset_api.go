package fail2ban

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anrted/opendeploy/pkg/contract"
)

type presetMetadata struct {
	title       string
	description string
	details     string
	icon        string
	blockedIPs  string
	exceptions  string
}

var protectionPresetMetadata = map[string]presetMetadata{
	"sshd": {
		title: "SSH Protection", icon: "terminal",
		description: "Protects SSH from password brute-force attacks.",
		details:     "Detects repeated authentication failures reported by the standard Fail2Ban sshd filter and bans the source address on the configured SSH port.",
		blockedIPs:  "Addresses producing repeated failed SSH authentication attempts.",
		exceptions:  "Addresses and networks listed in Ignore IP are never banned.",
	},
	"nginx_scanners": {
		title: "Nginx Scan Protection", icon: "search",
		description: "Blocks repeated scans for sensitive files and administrative paths.",
		details:     "Examines Nginx access logs for WordPress, repository, environment, vendor, backup, traversal and common scanner probes.",
		blockedIPs:  "Addresses repeatedly requesting protected paths or traversal payloads.",
		exceptions:  "Successful normal application requests and Ignore IP entries.",
	},
	"nginx_auth": {
		title: "Nginx Auth Protection", icon: "lock",
		description: "Stops repeated HTTP authentication failures.",
		details:     "Uses the distribution nginx-http-auth filter against the Nginx error log for HTTP Basic Auth and protected administration endpoints.",
		blockedIPs:  "Addresses repeatedly failing Nginx HTTP authentication.",
		exceptions:  "Ignore IP entries and requests that do not produce authentication failures.",
	},
	"nginx_bad_bots": {
		title: "Nginx Bad Bot Protection", icon: "crosshair",
		description: "Immediately blocks clients identifying as offensive scanners.",
		details:     "Matches explicit offensive scanner user agents such as masscan, zgrab, sqlmap, nikto and foda-scanner in the access log.",
		blockedIPs:  "Addresses whose user agent explicitly identifies a supported offensive scanner.",
		exceptions:  "Normal browser user agents and Ignore IP entries.",
	},
	"php_probes": {
		title: "PHP Exploit Protection", icon: "code",
		description: "Blocks probes for exposed PHP tools, shells and backup files.",
		details:     "Detects repeated requests for PHP shells, PHPUnit, installer, environment, database, archive and framework artefacts in the Nginx access log.",
		blockedIPs:  "Addresses repeatedly probing PHP exploit paths or missing PHP files.",
		exceptions:  "Successful normal PHP requests and Ignore IP entries.",
	},
}

var durationPattern = regexp.MustCompile(`^(?:-1|[1-9][0-9]*[smhdwy]?)$`)
var backendPattern = regexp.MustCompile(`^(?:auto|systemd|polling)$`)
var actionPattern = regexp.MustCompile(`^[A-Za-z0-9_()%.,:+-]+$`)
var portPattern = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9_-]*|[1-9][0-9]{0,4})(?:,(?:[A-Za-z][A-Za-z0-9_-]*|[1-9][0-9]{0,4}))*$`)

func managedPresetIDs() []string {
	ids := make([]string, 0, len(protectionPresetMetadata))
	for id := range protectionPresetMetadata {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func parseJail(content string) (string, map[string]string) {
	values := make(map[string]string)
	jail := ""
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			jail = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			values[strings.TrimSpace(strings.ToLower(key))] = strings.TrimSpace(value)
		}
	}
	return jail, values
}

func boolValue(value any, fallback bool) (bool, error) {
	if value == nil {
		return fallback, nil
	}
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		parsed, err := strconv.ParseBool(typed)
		if err == nil {
			return parsed, nil
		}
	}
	return false, fmt.Errorf("must be a boolean")
}

func stringValue(settings map[string]any, key, fallback string) string {
	value, ok := settings[key]
	if !ok || value == nil {
		return fallback
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func normalizedPresetSettings(base string, input map[string]any) (map[string]any, error) {
	_, defaults := parseJail(base)
	maxRetryDefault, _ := strconv.Atoi(defaults["maxretry"])
	result := map[string]any{
		"bantime": defaults["bantime"], "findtime": defaults["findtime"],
		"maxretry": maxRetryDefault, "backend": defaults["backend"],
		"logpath": defaults["logpath"], "ignoreip": defaults["ignoreip"],
		"banaction": defaults["banaction"], "port": defaults["port"],
		"ipv6": true, "auto_reload": true,
	}
	for key := range result {
		if value, ok := input[key]; ok {
			result[key] = value
		}
	}
	if err := validatePresetSettings(result); err != nil {
		return nil, err
	}
	return result, nil
}

func validatePresetSettings(result map[string]any) error {
	for _, key := range []string{"bantime", "findtime"} {
		value := stringValue(result, key, "")
		if !durationPattern.MatchString(value) || (key == "findtime" && value == "-1") {
			return fmt.Errorf("%s must be a positive Fail2Ban duration", key)
		}
		result[key] = value
	}
	maxRetry, err := strconv.Atoi(stringValue(result, "maxretry", ""))
	if err != nil || maxRetry < 1 || maxRetry > 1000 {
		return fmt.Errorf("maxretry must be between 1 and 1000")
	}
	result["maxretry"] = maxRetry
	backend := stringValue(result, "backend", "")
	if !backendPattern.MatchString(backend) {
		return fmt.Errorf("backend must be auto, systemd or polling")
	}
	result["backend"] = backend
	logPath := stringValue(result, "logpath", "")
	if logPath != "" && (!strings.HasPrefix(logPath, "/var/log/") || strings.Contains(logPath, "..") || strings.ContainsAny(logPath, "\r\n")) {
		return fmt.Errorf("logpath must be a safe path below /var/log")
	}
	result["logpath"] = logPath
	action := stringValue(result, "banaction", "")
	if action != "" && !actionPattern.MatchString(action) {
		return fmt.Errorf("banaction contains unsupported characters")
	}
	result["banaction"] = action
	if err := validatePresetPort(result); err != nil {
		return err
	}
	if err := validatePresetIgnoreIP(result); err != nil {
		return err
	}
	for _, key := range []string{"ipv6", "auto_reload"} {
		value, boolErr := boolValue(result[key], true)
		if boolErr != nil {
			return fmt.Errorf("%s %w", key, boolErr)
		}
		result[key] = value
	}
	return nil
}

func validatePresetPort(result map[string]any) error {
	port := stringValue(result, "port", "")
	if port != "" && !portPattern.MatchString(port) {
		return fmt.Errorf("port must be a service name, port number or comma-separated list")
	}
	for _, part := range strings.Split(port, ",") {
		if number, numberErr := strconv.Atoi(part); numberErr == nil && number > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}
	}
	result["port"] = port
	return nil
}

func validatePresetIgnoreIP(result map[string]any) error {
	ignoreIP := strings.Fields(stringValue(result, "ignoreip", ""))
	for _, address := range ignoreIP {
		if net.ParseIP(address) == nil {
			if _, _, cidrErr := net.ParseCIDR(address); cidrErr != nil {
				return fmt.Errorf("ignoreip contains invalid address or network %q", address)
			}
		}
	}
	result["ignoreip"] = strings.Join(ignoreIP, " ")
	return nil
}

func renderPresetJail(base string, settings map[string]any) string {
	lines := strings.Split(strings.TrimRight(base, "\n"), "\n")
	updates := map[string]string{
		"bantime": fmt.Sprint(settings["bantime"]), "findtime": fmt.Sprint(settings["findtime"]),
		"maxretry": fmt.Sprint(settings["maxretry"]), "backend": fmt.Sprint(settings["backend"]),
		"logpath": fmt.Sprint(settings["logpath"]), "ignoreip": fmt.Sprint(settings["ignoreip"]),
		"banaction": fmt.Sprint(settings["banaction"]), "port": fmt.Sprint(settings["port"]),
	}
	seen := make(map[string]bool)
	for index, raw := range lines {
		key, _, ok := strings.Cut(strings.TrimSpace(raw), "=")
		key = strings.ToLower(strings.TrimSpace(key))
		value, managed := updates[key]
		if !ok || !managed {
			continue
		}
		seen[key] = true
		if value == "" {
			lines[index] = ""
		} else {
			lines[index] = key + " = " + value
		}
	}
	for _, key := range []string{"banaction", "ignoreip", "logpath", "port"} {
		if value := updates[key]; value != "" && !seen[key] {
			lines = append(lines, key+" = "+value)
		}
	}
	var compact []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			compact = append(compact, line)
		}
	}
	return strings.Join(compact, "\n") + "\n"
}

func filterRuleCount(content string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "failregex =") || strings.HasPrefix(line, "^") {
			count++
		}
	}
	return count
}

func (m *Module) currentPreset(ctx context.Context, id string) (protectionPreset, string, bool, *time.Time, error) {
	preset, ok := protectionPresets[id]
	if !ok || id == "manual" {
		return protectionPreset{}, "", false, nil, fmt.Errorf("unknown protection preset: %s", id)
	}
	content, err := m.agent.FileRead(ctx, preset.jailPath)
	enabled := err == nil
	if !enabled {
		content, err = m.agent.FileRead(ctx, preset.jailPath+".disabled")
	}
	if err != nil {
		content = []byte(preset.jailContent)
	}
	var modified *time.Time
	if entries, listErr := m.agent.DirList(ctx, filepath.Dir(preset.jailPath)); listErr == nil {
		for _, entry := range entries {
			if entry.Path == preset.jailPath || entry.Path == preset.jailPath+".disabled" || entry.Name == filepath.Base(preset.jailPath) {
				value := entry.ModTime
				modified = &value
				break
			}
		}
	}
	return preset, string(content), enabled, modified, nil
}

func (m *Module) ProtectionPresets(ctx context.Context) ([]contract.ProtectionPreset, error) {
	result := make([]contract.ProtectionPreset, 0, len(protectionPresetMetadata))
	for _, id := range managedPresetIDs() {
		preset, content, enabled, modified, err := m.currentPreset(ctx, id)
		if err != nil {
			return nil, err
		}
		jail, raw := parseJail(content)
		settings, settingsErr := normalizedPresetSettings(preset.jailContent, mapStringAny(raw))
		if settingsErr != nil {
			settings, _ = normalizedPresetSettings(preset.jailContent, nil)
		}
		defaults, _ := normalizedPresetSettings(preset.jailContent, nil)
		meta := protectionPresetMetadata[id]
		files := []string{preset.jailPath}
		filters := []string{raw["filter"]}
		if preset.filterPath != "" {
			files = append(files, preset.filterPath)
		}
		result = append(result, contract.ProtectionPreset{
			ID: id, Title: meta.title, Description: meta.description, Icon: meta.icon,
			Enabled: enabled, NeedsUpdate: enabled && presetNeedsUpdate(ctx, m.agent, protectionPreset{filterPath: preset.filterPath, filterContent: preset.filterContent, jailPath: preset.jailPath, jailContent: renderPresetJail(preset.jailContent, settings)}),
			Jails: []string{jail}, RuleCount: max(1, filterRuleCount(preset.filterContent)),
			LastModified: modified, Settings: settings, Defaults: defaults, Files: files,
			Filters: filters, LogPaths: compactStrings(raw["logpath"]), Actions: displayBanActions(raw["banaction"]),
			BlockedIPs: meta.blockedIPs, Exceptions: meta.exceptions,
		})
	}
	return result, nil
}

func mapStringAny(input map[string]string) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func compactStrings(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	return []string{value}
}

func displayBanActions(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{"Fail2Ban default"}
	}
	return []string{value}
}

func (m *Module) PreviewProtectionPreset(ctx context.Context, id string, input map[string]any) (*contract.ProtectionPresetPreview, error) {
	preset, current, _, _, err := m.currentPreset(ctx, id)
	if err != nil {
		return nil, err
	}
	settings, err := normalizedPresetSettings(current, input)
	if err != nil {
		return nil, err
	}
	jail, _ := parseJail(preset.jailContent)
	files := []string{preset.jailPath}
	if preset.filterPath != "" {
		files = append(files, preset.filterPath)
	}
	return &contract.ProtectionPresetPreview{
		PresetID: id, Jails: []string{jail}, Parameters: settings, Files: files,
		Services: []string{"fail2ban"}, Configuration: renderPresetJail(preset.jailContent, settings),
		Filter: preset.filterContent,
	}, nil
}

func (m *Module) SaveProtectionPreset(ctx context.Context, id string, input map[string]any) error {
	preset, current, enabled, _, err := m.currentPreset(ctx, id)
	if err != nil {
		return err
	}
	settings, err := normalizedPresetSettings(current, input)
	if err != nil {
		return err
	}
	target := preset.jailPath + ".disabled"
	if enabled {
		target = preset.jailPath
	}
	return m.applyPresetFiles(ctx, id, preset, target, renderPresetJail(preset.jailContent, settings), enabled, settings["auto_reload"] == true)
}

func (m *Module) ResetProtectionPreset(ctx context.Context, id string) error {
	preset, _, enabled, _, err := m.currentPreset(ctx, id)
	if err != nil {
		return err
	}
	target := preset.jailPath + ".disabled"
	if enabled {
		target = preset.jailPath
	}
	return m.applyPresetFiles(ctx, id, preset, target, preset.jailContent, enabled, true)
}

func (m *Module) SetProtectionPresetEnabled(ctx context.Context, id string, enabled bool) error {
	preset, current, currentEnabled, _, err := m.currentPreset(ctx, id)
	if err != nil {
		return err
	}
	if enabled == currentEnabled {
		return nil
	}
	if enabled {
		settings, normalizeErr := normalizedPresetSettings(current, nil)
		if normalizeErr != nil {
			return normalizeErr
		}
		return m.applyPresetFiles(ctx, id, preset, preset.jailPath, renderPresetJail(preset.jailContent, settings), true, true)
	}
	return m.disableProtectionPreset(ctx, id)
}

func (m *Module) applyPresetFiles(ctx context.Context, id string, preset protectionPreset, jailPath, jailContent string, reload, autoReload bool) error {
	filterSnapshot := m.snapshotConfig(ctx, preset.filterPath)
	jailSnapshot := m.snapshotConfig(ctx, jailPath)
	if preset.filterPath != "" {
		if err := m.agent.FileWrite(ctx, preset.filterPath, []byte(preset.filterContent), 0o644); err != nil {
			return fmt.Errorf("write %s filter: %w", id, err)
		}
	}
	if err := m.agent.FileWrite(ctx, jailPath, []byte(jailContent), 0o644); err != nil {
		m.restoreConfig(ctx, filterSnapshot)
		return fmt.Errorf("write %s jail: %w", id, err)
	}
	if !reload {
		return nil
	}
	exitCode, _, stderr, testErr := m.agent.CommandExecute(ctx, "fail2ban-server", "-t")
	if testErr != nil || exitCode != 0 {
		m.restoreConfig(ctx, filterSnapshot)
		m.restoreConfig(ctx, jailSnapshot)
		return fmt.Errorf("validate %s protection: %s", id, strings.TrimSpace(stderr))
	}
	if autoReload {
		if err := m.agent.ServiceRestart(ctx, "fail2ban"); err != nil {
			m.restoreConfig(ctx, filterSnapshot)
			m.restoreConfig(ctx, jailSnapshot)
			_ = m.agent.ServiceRestart(ctx, "fail2ban")
			return fmt.Errorf("apply %s protection: %w", id, err)
		}
	}
	if err := m.agent.ServiceEnable(ctx, "fail2ban"); err != nil {
		m.restoreConfig(ctx, filterSnapshot)
		m.restoreConfig(ctx, jailSnapshot)
		_ = m.agent.ServiceRestart(ctx, "fail2ban")
		return fmt.Errorf("enable fail2ban at boot: %w", err)
	}
	return nil
}
