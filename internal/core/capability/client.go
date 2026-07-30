// Package capability routes the stable AgentClient contract to the selected
// server. Callers do not distinguish local and remote execution.
package capability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/anrted/opendeploy/internal/core/controlplane"
	"github.com/anrted/opendeploy/internal/core/servercontext"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/contract"
)

type Client struct {
	local  contract.AgentClient
	remote *controlplane.Manager
}

func NewClient(local contract.AgentClient, remote *controlplane.Manager) *Client {
	return &Client{local: local, remote: remote}
}

func remoteError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, controlplane.ErrAgentOffline),
		errors.Is(err, controlplane.ErrConnectionClosed):
		return apperrors.AgentUnavailable(err)
	case errors.Is(err, controlplane.ErrCapabilityAbsent):
		return apperrors.Wrap(
			http.StatusNotImplemented,
			apperrors.CodeCapabilityUnavailable,
			"the selected Agent does not support this capability",
			err,
		)
	case errors.Is(err, context.DeadlineExceeded):
		return apperrors.Wrap(
			http.StatusGatewayTimeout,
			apperrors.CodeAgentTimeout,
			"the selected Agent did not respond before the deadline",
			err,
		)
	default:
		return err
	}
}

func (c *Client) call(ctx context.Context, kind string, request, response any) error {
	if servercontext.IsLocal(ctx) {
		return fmt.Errorf("local capability %q must use its typed adapter", kind)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	result, err := c.remote.Dispatch(ctx, servercontext.ID(ctx), kind, payload)
	if err != nil {
		return remoteError(err)
	}
	if response == nil || len(result) == 0 {
		return nil
	}
	return json.Unmarshal(result, response)
}

func (c *Client) ServiceStart(ctx context.Context, name string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.ServiceStart(ctx, name)
	}
	return c.call(ctx, "service.action", map[string]any{"name": name, "action": "start"}, nil)
}
func (c *Client) ServiceStop(ctx context.Context, name string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.ServiceStop(ctx, name)
	}
	return c.call(ctx, "service.action", map[string]any{"name": name, "action": "stop"}, nil)
}
func (c *Client) ServiceRestart(ctx context.Context, name string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.ServiceRestart(ctx, name)
	}
	return c.call(ctx, "service.action", map[string]any{"name": name, "action": "restart"}, nil)
}
func (c *Client) ServiceEnable(ctx context.Context, name string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.ServiceEnable(ctx, name)
	}
	return c.call(ctx, "service.action", map[string]any{"name": name, "action": "enable"}, nil)
}
func (c *Client) ServiceDisable(ctx context.Context, name string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.ServiceDisable(ctx, name)
	}
	return c.call(ctx, "service.action", map[string]any{"name": name, "action": "disable"}, nil)
}
func (c *Client) ServiceReload(ctx context.Context, name string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.ServiceReload(ctx, name)
	}
	return c.call(ctx, "service.action", map[string]any{"name": name, "action": "reload"}, nil)
}
func (c *Client) ServiceStatus(ctx context.Context, name string) (*contract.ServiceStatus, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.ServiceStatus(ctx, name)
	}
	var result contract.ServiceStatus
	err := c.call(ctx, "service.status", map[string]string{"name": name}, &result)
	return &result, err
}
func (c *Client) ServiceLogs(ctx context.Context, name string, lines int) ([]string, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.ServiceLogs(ctx, name, lines)
	}
	var result []string
	err := c.call(ctx, "service.logs", map[string]any{"name": name, "lines": lines}, &result)
	return result, err
}
func (c *Client) ServiceStreamLogs(ctx context.Context, name string) (<-chan string, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.ServiceStreamLogs(ctx, name)
	}
	payload, _ := json.Marshal(map[string]string{"name": name})
	return c.stringStream(ctx, "service.logs", payload)
}
func (c *Client) FileLogs(ctx context.Context, path string, lines int) ([]string, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.FileLogs(ctx, path, lines)
	}
	var result []string
	err := c.call(ctx, "file.logs", map[string]any{"path": path, "lines": lines}, &result)
	return result, err
}
func (c *Client) StreamLogs(ctx context.Context, path string) (<-chan string, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.StreamLogs(ctx, path)
	}
	payload, _ := json.Marshal(map[string]string{"path": path})
	return c.stringStream(ctx, "file.logs", payload)
}

func (c *Client) stringStream(ctx context.Context, kind string, payload []byte) (<-chan string, error) {
	chunks, err := c.remote.Subscribe(ctx, servercontext.ID(ctx), kind, payload)
	if err != nil {
		return nil, remoteError(err)
	}
	output := make(chan string, 64)
	go func() {
		defer close(output)
		for chunk := range chunks {
			select {
			case output <- string(chunk):
			case <-ctx.Done():
				return
			}
		}
	}()
	return output, nil
}
func linesChannel(lines []string) <-chan string {
	output := make(chan string, len(lines))
	for _, line := range lines {
		output <- line
	}
	close(output)
	return output
}
func (c *Client) CommandExecute(ctx context.Context, command string, args ...string) (int, string, string, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.CommandExecute(ctx, command, args...)
	}
	var result struct {
		ExitCode       int `json:"exit_code"`
		Stdout, Stderr string
	}
	err := c.call(ctx, "command.execute", map[string]any{"command": command, "args": args}, &result)
	return result.ExitCode, result.Stdout, result.Stderr, err
}
func (c *Client) packageOperation(ctx context.Context, action, pkg string) (<-chan string, error) {
	var result []string
	err := c.call(ctx, "package."+action, map[string]string{"package": pkg}, &result)
	return linesChannel(result), err
}
func (c *Client) PackageInstall(ctx context.Context, pkg string) (<-chan string, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.PackageInstall(ctx, pkg)
	}
	return c.packageOperation(ctx, "install", pkg)
}
func (c *Client) PackageRemove(ctx context.Context, pkg string) (<-chan string, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.PackageRemove(ctx, pkg)
	}
	return c.packageOperation(ctx, "remove", pkg)
}
func (c *Client) PackageUpdate(ctx context.Context, pkg string) (<-chan string, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.PackageUpdate(ctx, pkg)
	}
	return c.packageOperation(ctx, "update", pkg)
}
func (c *Client) PackageInstalled(ctx context.Context, pkg string) (bool, string, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.PackageInstalled(ctx, pkg)
	}
	var result struct {
		Installed bool   `json:"installed"`
		Version   string `json:"version"`
	}
	err := c.call(ctx, "package.status", map[string]string{"package": pkg}, &result)
	return result.Installed, result.Version, err
}
func (c *Client) FileRead(ctx context.Context, path string) ([]byte, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.FileRead(ctx, path)
	}
	var result []byte
	err := c.call(ctx, "file.read", map[string]string{"path": path}, &result)
	return result, err
}
func (c *Client) FileWrite(ctx context.Context, path string, content []byte, mode uint32) error {
	if servercontext.IsLocal(ctx) {
		return c.local.FileWrite(ctx, path, content, mode)
	}
	return c.call(ctx, "file.write", map[string]any{"path": path, "content": content, "mode": mode}, nil)
}
func (c *Client) FileDelete(ctx context.Context, path string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.FileDelete(ctx, path)
	}
	return c.call(ctx, "file.delete", map[string]string{"path": path}, nil)
}
func (c *Client) FileRename(ctx context.Context, oldPath, newPath string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.FileRename(ctx, oldPath, newPath)
	}
	return c.call(ctx, "file.rename", map[string]string{"old_path": oldPath, "new_path": newPath}, nil)
}
func (c *Client) FileCopy(ctx context.Context, srcPath, dstPath string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.FileCopy(ctx, srcPath, dstPath)
	}
	return c.call(ctx, "file.copy", map[string]string{"src_path": srcPath, "dst_path": dstPath}, nil)
}
func (c *Client) FileChmod(ctx context.Context, path string, mode uint32) error {
	if servercontext.IsLocal(ctx) {
		return c.local.FileChmod(ctx, path, mode)
	}
	return c.call(ctx, "file.chmod", map[string]any{"path": path, "mode": mode}, nil)
}
func (c *Client) FileChown(ctx context.Context, path string, uid, gid int) error {
	if servercontext.IsLocal(ctx) {
		return c.local.FileChown(ctx, path, uid, gid)
	}
	return c.call(ctx, "file.chown", map[string]any{"path": path, "uid": uid, "gid": gid}, nil)
}
func (c *Client) DirCreate(ctx context.Context, path string, mode uint32) error {
	if servercontext.IsLocal(ctx) {
		return c.local.DirCreate(ctx, path, mode)
	}
	return c.call(ctx, "directory.create", map[string]any{"path": path, "mode": mode}, nil)
}
func (c *Client) DirList(ctx context.Context, path string) ([]contract.FileInfo, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.DirList(ctx, path)
	}
	var result []contract.FileInfo
	err := c.call(ctx, "directory.list", map[string]string{"path": path}, &result)
	return result, err
}
func (c *Client) ArchiveCreate(ctx context.Context, paths []string, dstPath string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.ArchiveCreate(ctx, paths, dstPath)
	}
	return c.call(ctx, "archive.create", map[string]any{"paths": paths, "dst_path": dstPath}, nil)
}
func (c *Client) ArchiveExtract(ctx context.Context, srcPath, dstDir string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.ArchiveExtract(ctx, srcPath, dstDir)
	}
	return c.call(ctx, "archive.extract", map[string]string{"src_path": srcPath, "dst_dir": dstDir}, nil)
}
func (c *Client) FirewallStatus(ctx context.Context) (*contract.FirewallStatus, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.FirewallStatus(ctx)
	}
	var result contract.FirewallStatus
	err := c.call(ctx, "firewall.status", struct{}{}, &result)
	return &result, err
}
func (c *Client) FirewallRule(ctx context.Context, req *contract.FirewallRuleRequest) error {
	if servercontext.IsLocal(ctx) {
		return c.local.FirewallRule(ctx, req)
	}
	return c.call(ctx, "firewall.rule", req, nil)
}
func (c *Client) FirewallDelete(ctx context.Context, id string) error {
	if servercontext.IsLocal(ctx) {
		return c.local.FirewallDelete(ctx, id)
	}
	return c.call(ctx, "firewall.delete", map[string]string{"id": id}, nil)
}
func (c *Client) FirewallList(ctx context.Context) ([]*contract.FirewallRule, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.FirewallList(ctx)
	}
	var result []*contract.FirewallRule
	err := c.call(ctx, "firewall.list", struct{}{}, &result)
	return result, err
}
func (c *Client) FirewallToggle(ctx context.Context, enable bool) error {
	if servercontext.IsLocal(ctx) {
		return c.local.FirewallToggle(ctx, enable)
	}
	return c.call(ctx, "firewall.toggle", map[string]bool{"enable": enable}, nil)
}
func (c *Client) FirewallReset(ctx context.Context) error {
	if servercontext.IsLocal(ctx) {
		return c.local.FirewallReset(ctx)
	}
	return c.call(ctx, "firewall.reset", struct{}{}, nil)
}
func (c *Client) SystemStats(ctx context.Context) (*contract.SystemStats, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.SystemStats(ctx)
	}
	var result contract.SystemStats
	err := c.call(ctx, "system.stats", struct{}{}, &result)
	return &result, err
}
func (c *Client) ProcessList(ctx context.Context) ([]contract.ProcessStats, error) {
	if servercontext.IsLocal(ctx) {
		return c.local.ProcessList(ctx)
	}
	var result []contract.ProcessStats
	err := c.call(ctx, "process.list", struct{}{}, &result)
	return result, err
}
func (c *Client) ProcessKill(ctx context.Context, pid int, force bool) error {
	if servercontext.IsLocal(ctx) {
		return c.local.ProcessKill(ctx, pid, force)
	}
	return c.call(ctx, "process.kill", map[string]any{"pid": pid, "force": force}, nil)
}

var _ contract.AgentClient = (*Client)(nil)
