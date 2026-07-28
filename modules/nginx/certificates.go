package nginx

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (m *Module) certificateRows(ctx context.Context) ([]map[string]any, error) {
	entries, err := m.deps.Agent.DirList(ctx, "/etc/nginx/sites-available")
	if err != nil {
		return nil, fmt.Errorf("list nginx sites for certificates: %w", err)
	}

	rows := make([]map[string]any, 0)
	seen := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir || !strings.HasPrefix(entry.Name, "opendeploy-") || !strings.HasSuffix(entry.Name, ".conf") {
			continue
		}
		content, err := m.deps.Agent.FileRead(ctx, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("read nginx site %s: %w", entry.Name, err)
		}
		certificatePath := nginxDirective(string(content), "ssl_certificate")
		if certificatePath == "" {
			continue
		}
		if _, exists := seen[certificatePath]; exists {
			continue
		}
		seen[certificatePath] = struct{}{}

		domain := strings.TrimSuffix(strings.TrimPrefix(entry.Name, "opendeploy-"), ".conf")
		row := map[string]any{
			"domain":    domain,
			"path":      certificatePath,
			"provider":  certificateProvider(certificatePath),
			"issuer":    "",
			"issued_at": "",
			"expires":   "",
			"remaining": "",
			"san":       "",
			"status":    "invalid",
		}
		exitCode, stdout, stderr, commandErr := m.deps.Agent.CommandExecute(
			ctx, "openssl", "x509", "-in", certificatePath, "-noout",
			"-issuer", "-subject", "-dates", "-ext", "subjectAltName",
		)
		if commandErr != nil || exitCode != 0 {
			row["error"] = strings.TrimSpace(stderr)
			rows = append(rows, row)
			continue
		}
		applyCertificateMetadata(row, stdout, time.Now())
		rows = append(rows, row)
	}
	return rows, nil
}

func certificateProvider(certificatePath string) string {
	if strings.HasPrefix(certificatePath, "/etc/letsencrypt/") {
		return "Let's Encrypt"
	}
	return "Custom"
}

func applyCertificateMetadata(row map[string]any, output string, now time.Time) {
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case strings.HasPrefix(line, "issuer="):
			row["issuer"] = strings.TrimSpace(strings.TrimPrefix(line, "issuer="))
		case strings.HasPrefix(line, "notBefore="):
			row["issued_at"] = strings.TrimSpace(strings.TrimPrefix(line, "notBefore="))
		case strings.HasPrefix(line, "notAfter="):
			expiresText := strings.TrimSpace(strings.TrimPrefix(line, "notAfter="))
			row["expires"] = expiresText
			if expiresAt, err := time.Parse("Jan 2 15:04:05 2006 MST", expiresText); err == nil {
				days := int(expiresAt.Sub(now).Hours() / 24)
				row["remaining"] = fmt.Sprintf("%d days", days)
				switch {
				case !expiresAt.After(now):
					row["status"] = "expired"
				case expiresAt.Sub(now) < 30*24*time.Hour:
					row["status"] = "expiring"
				default:
					row["status"] = "valid"
				}
			}
		case strings.Contains(line, "DNS:"):
			row["san"] = line
		}
	}
}
