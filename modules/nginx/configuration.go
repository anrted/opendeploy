package nginx

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"
)

var nginxConfigurationRoots = []string{
	nginxMainConfigPath,
	"/etc/nginx/mime.types",
	"/etc/nginx/sites-available",
	"/etc/nginx/sites-enabled",
	"/etc/nginx/conf.d",
	"/etc/nginx/snippets",
}

func (m *Module) configurationRows(ctx context.Context) ([]map[string]any, error) {
	rows := make([]map[string]any, 0)
	for _, root := range nginxConfigurationRoots {
		if strings.HasSuffix(root, ".conf") || strings.HasSuffix(root, "mime.types") {
			content, err := m.deps.Agent.FileRead(ctx, root)
			if err != nil {
				continue
			}
			rows = append(rows, configurationRow(root, "file", time.Time{}, content))
			continue
		}
		entries, err := m.deps.Agent.DirList(ctx, root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir || !isEditableNginxConfig(entry.Path) {
				continue
			}
			content, err := m.deps.Agent.FileRead(ctx, entry.Path)
			if err != nil {
				continue
			}
			rows = append(rows, configurationRow(entry.Path, path.Base(root), entry.ModTime, content))
		}
	}
	return rows, nil
}

func configurationRow(filePath, kind string, modified time.Time, content []byte) map[string]any {
	modifiedText := ""
	if !modified.IsZero() {
		modifiedText = modified.UTC().Format(time.RFC3339)
	}
	preview := strings.TrimSpace(string(content))
	if len(preview) > 160 {
		preview = preview[:160] + "…"
	}
	return map[string]any{
		"path":     filePath,
		"kind":     kind,
		"modified": modifiedText,
		"content":  string(content),
		"preview":  preview,
	}
}

func (m *Module) saveConfigurationFile(ctx context.Context, filePath, content string) error {
	if !isEditableNginxConfig(filePath) {
		return fmt.Errorf("configuration path is not editable")
	}
	if strings.TrimSpace(content) == "" || len(content) > 1<<20 || strings.ContainsRune(content, '\x00') {
		return fmt.Errorf("configuration content is empty or too large")
	}
	snapshot := m.readSnapshot(ctx, filePath)
	if !snapshot.exists {
		return fmt.Errorf("configuration file does not exist")
	}
	if err := m.deps.Agent.FileWrite(ctx, filePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write nginx configuration: %w", err)
	}
	if err := m.validateAndReload(ctx); err != nil {
		m.restoreSnapshot(ctx, filePath, snapshot)
		_ = m.deps.Agent.ServiceReload(ctx, "nginx")
		return err
	}
	return nil
}

func isEditableNginxConfig(filePath string) bool {
	if filePath == nginxMainConfigPath || filePath == "/etc/nginx/mime.types" {
		return true
	}
	for _, root := range []string{
		"/etc/nginx/sites-available/",
		"/etc/nginx/conf.d/",
		"/etc/nginx/snippets/",
	} {
		if strings.HasPrefix(filePath, root) {
			name := strings.TrimPrefix(filePath, root)
			return name != "" && !strings.Contains(name, "/") &&
				(strings.HasSuffix(name, ".conf") || root == "/etc/nginx/snippets/")
		}
	}
	return false
}
