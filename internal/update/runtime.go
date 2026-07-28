package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type RuntimeController interface {
	Restart(ctx context.Context) error
	Healthy(ctx context.Context) error
}

type SystemRuntime struct {
	CoreHealthURL string
	Client        *http.Client
}

func (r *SystemRuntime) Restart(ctx context.Context) error {
	for _, unit := range []string{"opendeploy-agent.service", "opendeploy-core.service"} {
		output, err := exec.CommandContext(ctx, "systemctl", "restart", unit).CombinedOutput() //nolint:gosec // unit comes from fixed allowlist
		if err != nil {
			return fmt.Errorf("update: restart %s: %s: %w", unit, strings.TrimSpace(string(output)), err)
		}
	}
	return nil
}

func (r *SystemRuntime) Healthy(ctx context.Context) error {
	for _, unit := range []string{"opendeploy-agent.service", "opendeploy-core.service"} {
		output, err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unit).CombinedOutput() //nolint:gosec // unit comes from fixed allowlist
		if err != nil {
			return fmt.Errorf("update: service %s is unhealthy: %s", unit, strings.TrimSpace(string(output)))
		}
	}
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	endpoint := r.CoreHealthURL
	if endpoint == "" {
		endpoint = "http://127.0.0.1:5888/health"
	}
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		response, err := client.Do(request)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
			_ = response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("HTTP %d", response.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("update: core health check failed: %w", lastErr)
}
