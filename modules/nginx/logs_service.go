package nginx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anrted/opendeploy/pkg/contract"
)

// LogsService owns Nginx log discovery and access.
type LogsService struct {
	agent contract.AgentClient
}

func NewLogsService(agent contract.AgentClient) *LogsService {
	return &LogsService{agent: agent}
}

func (s *LogsService) Logs(ctx context.Context) []contract.LogDef {
	logs := []contract.LogDef{
		{ID: "service", Name: "Systemd Service Log", Type: "systemd"},
		{ID: "access", Name: "Global Access Log", Type: "file", Path: "/var/log/nginx/access.log"},
		{ID: "error", Name: "Global Error Log", Type: "file", Path: "/var/log/nginx/error.log"},
	}
	if ctx == nil {
		ctx = context.Background()
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	entries, err := s.agent.DirList(discoveryCtx, "/var/log/nginx")
	if err != nil {
		return logs
	}
	for _, entry := range entries {
		if entry.IsDir || !strings.HasSuffix(entry.Name, ".log") ||
			entry.Name == "access.log" || entry.Name == "error.log" {
			continue
		}
		logs = append(logs, contract.LogDef{
			ID: strings.TrimSuffix(entry.Name, ".log"), Name: entry.Name, Type: "file", Path: entry.Path,
		})
	}
	return logs
}

func (s *LogsService) Read(ctx context.Context, logID string, lines int) ([]string, error) {
	if logID == "service" {
		return s.agent.ServiceLogs(ctx, "nginx", lines)
	}
	path, err := s.path(ctx, logID)
	if err != nil {
		return nil, err
	}
	return s.agent.FileLogs(ctx, path, lines)
}

func (s *LogsService) Clear(ctx context.Context, logID string) error {
	if logID == "service" {
		return fmt.Errorf("cannot clear systemd service logs directly")
	}
	path, err := s.path(ctx, logID)
	if err != nil {
		return err
	}
	return s.agent.FileWrite(ctx, path, nil, 0o640)
}

func (s *LogsService) path(ctx context.Context, logID string) (string, error) {
	for _, log := range s.Logs(ctx) {
		if log.ID == logID && log.Type == "file" {
			return log.Path, nil
		}
	}
	return "", fmt.Errorf("log %s not found", logID)
}
