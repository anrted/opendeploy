package remote

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/anrted/opendeploy/internal/agent/stats"
	"github.com/anrted/opendeploy/internal/platform/config"
)

type task struct {
	ID     string `json:"id"`
	Action string `json:"action"`
}

type taskResult struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Output string `json:"output"`
	Error  string `json:"error"`
}

type heartbeatResponse struct {
	HeartbeatInterval int    `json:"heartbeat_interval"`
	Tasks             []task `json:"tasks"`
}

type Client struct {
	cfg       config.AgentConfig
	http      *http.Client
	collector *stats.Collector
	logger    *slog.Logger
	pending   []taskResult
}

func New(cfg config.AgentConfig, logger *slog.Logger) (*Client, error) {
	if cfg.CoreURL == "" || cfg.ServerID == "" || cfg.CertificateFingerprint == "" {
		return nil, fmt.Errorf("remote agent configuration is incomplete")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.CertificateFile != "" && cfg.PrivateKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertificateFile, cfg.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load agent certificate: %w", err)
		}
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
	}
	return &Client{
		cfg: cfg, http: &http.Client{Transport: transport, Timeout: 20 * time.Second},
		collector: stats.NewCollector(), logger: logger,
	}, nil
}

func (c *Client) Run(ctx context.Context) {
	interval := c.cfg.HeartbeatInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			next, err := c.heartbeat(ctx)
			if err != nil {
				c.logger.WarnContext(ctx, "remote heartbeat failed", "error", err)
			} else if next > 0 {
				interval = next
			}
			timer.Reset(interval)
		}
	}
}

func (c *Client) heartbeat(ctx context.Context) (time.Duration, error) {
	snapshot, err := c.collector.Collect()
	if err != nil {
		return 0, err
	}
	diskUsage := 0.0
	for _, disk := range snapshot.Disk {
		if disk.UsedPercent > diskUsage {
			diskUsage = disk.UsedPercent
		}
	}
	payload := map[string]any{
		"state": "online", "cpu_usage": snapshot.CPU.UsagePercent,
		"memory_usage": snapshot.Memory.UsedPercent, "disk_usage": diskUsage,
		"uptime": snapshot.Uptime, "running_tasks": 0, "task_results": c.pending,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.CoreURL, "/")+"/api/v1/agents/heartbeat", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenDeploy-Agent-ID", c.cfg.ServerID)
	req.Header.Set("X-OpenDeploy-Cert-Fingerprint", c.cfg.CertificateFingerprint)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("core returned HTTP %d", resp.StatusCode)
	}
	var result heartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	c.pending = c.execute(result.Tasks)
	return time.Duration(result.HeartbeatInterval) * time.Second, nil
}

func (c *Client) execute(tasks []task) []taskResult {
	results := make([]taskResult, 0, len(tasks))
	for _, current := range tasks {
		result := taskResult{ID: current.ID, State: "success"}
		switch current.Action {
		case "health_check", "refresh_information", "reconnect":
			result.Output = "agent is healthy"
		default:
			result.State = "error"
			result.Error = "action requires a supervised agent update or host power controller"
		}
		results = append(results, result)
	}
	return results
}
