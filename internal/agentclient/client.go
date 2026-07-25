// Package agentclient adapts the generated Agent gRPC client to the stable
// contract.AgentClient interface consumed by Core and modules.
package agentclient

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/anrted/opendeploy/pkg/contract"
	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
)

type Client struct {
	conn   *grpc.ClientConn
	stub   agentv1.AgentServiceClient
	logger *slog.Logger
}

func Dial(addr string, timeout time.Duration, logger *slog.Logger) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second, Timeout: 10 * time.Second, PermitWithoutStream: true}),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("agentclient: dial %q: %w", addr, err)
	}
	client := &Client{conn: conn, stub: agentv1.NewAgentServiceClient(conn), logger: logger}
	if _, err := client.stub.Ping(ctx, &agentv1.PingRequest{}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("agentclient: ping: %w", err)
	}
	logger.Info("agentclient: connected to agent", "addr", addr)
	return client, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) ServiceStart(ctx context.Context, name string) error {
	return c.serviceAction(ctx, name, agentv1.ServiceActionType_SERVICE_ACTION_START)
}
func (c *Client) ServiceStop(ctx context.Context, name string) error {
	return c.serviceAction(ctx, name, agentv1.ServiceActionType_SERVICE_ACTION_STOP)
}
func (c *Client) ServiceRestart(ctx context.Context, name string) error {
	return c.serviceAction(ctx, name, agentv1.ServiceActionType_SERVICE_ACTION_RESTART)
}
func (c *Client) ServiceEnable(ctx context.Context, name string) error {
	return c.serviceAction(ctx, name, agentv1.ServiceActionType_SERVICE_ACTION_ENABLE)
}
func (c *Client) ServiceDisable(ctx context.Context, name string) error {
	return c.serviceAction(ctx, name, agentv1.ServiceActionType_SERVICE_ACTION_DISABLE)
}

func (c *Client) serviceAction(ctx context.Context, name string, action agentv1.ServiceActionType) error {
	_, err := c.stub.ServiceAction(ctx, &agentv1.ServiceActionRequest{ServiceName: name, Action: action})
	return err
}

func (c *Client) ServiceStatus(ctx context.Context, name string) (*contract.ServiceStatus, error) {
	result, err := c.stub.ServiceStatus(ctx, &agentv1.ServiceStatusRequest{ServiceName: name})
	if err != nil {
		return nil, err
	}
	return &contract.ServiceStatus{Name: result.Name, Active: result.Active, Enabled: result.Enabled, SubState: result.SubState, Description: result.Description}, nil
}

func (c *Client) ServiceLogs(ctx context.Context, name string, lines int) ([]string, error) {
	stream, err := c.stub.ServiceLogs(ctx, &agentv1.ServiceLogsRequest{ServiceName: name, Lines: int32(lines)})
	if err != nil {
		return nil, err
	}
	var result []string
	for {
		line, err := stream.Recv()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		result = append(result, line.Line)
	}
}

func (c *Client) PackageInstall(ctx context.Context, pkg string) (<-chan string, error) {
	stream, err := c.stub.PackageInstall(ctx, &agentv1.PackageRequest{PackageName: pkg})
	return packageOutput(stream, err)
}
func (c *Client) PackageRemove(ctx context.Context, pkg string) (<-chan string, error) {
	stream, err := c.stub.PackageRemove(ctx, &agentv1.PackageRequest{PackageName: pkg})
	return packageOutput(stream, err)
}
func (c *Client) PackageUpdate(ctx context.Context, pkg string) (<-chan string, error) {
	stream, err := c.stub.PackageUpdate(ctx, &agentv1.PackageRequest{PackageName: pkg})
	return packageOutput(stream, err)
}

type outputStream interface {
	Recv() (*agentv1.PackageOutput, error)
}

func packageOutput(stream outputStream, err error) (<-chan string, error) {
	if err != nil {
		return nil, err
	}
	output := make(chan string, 64)
	go func() {
		defer close(output)
		for {
			line, recvErr := stream.Recv()
			if recvErr != nil {
				return
			}
			output <- line.Line
		}
	}()
	return output, nil
}

func (c *Client) PackageInstalled(ctx context.Context, pkg string) (bool, string, error) {
	result, err := c.stub.PackageStatus(ctx, &agentv1.PackageRequest{PackageName: pkg})
	if err != nil {
		return false, "", err
	}
	return result.Installed, result.Version, nil
}

func (c *Client) FileRead(ctx context.Context, path string) ([]byte, error) {
	result, err := c.stub.FileRead(ctx, &agentv1.FileReadRequest{Path: path})
	if err != nil {
		return nil, err
	}
	return result.Content, nil
}
func (c *Client) FileWrite(ctx context.Context, path string, content []byte, mode uint32) error {
	_, err := c.stub.FileWrite(ctx, &agentv1.FileWriteRequest{Path: path, Content: content, Mode: mode})
	return err
}
func (c *Client) FileDelete(ctx context.Context, path string) error {
	_, err := c.stub.FileDelete(ctx, &agentv1.FileDeleteRequest{Path: path})
	return err
}
func (c *Client) DirCreate(ctx context.Context, path string, mode uint32) error {
	_, err := c.stub.DirCreate(ctx, &agentv1.DirCreateRequest{Path: path, Mode: mode})
	return err
}
func (c *Client) DirList(ctx context.Context, path string) ([]contract.FileInfo, error) {
	result, err := c.stub.DirList(ctx, &agentv1.DirListRequest{Path: path})
	if err != nil {
		return nil, err
	}
	entries := make([]contract.FileInfo, 0, len(result.Entries))
	for _, entry := range result.Entries {
		entries = append(entries, contract.FileInfo{Name: entry.Name, Path: entry.Path, IsDir: entry.IsDir, Size: entry.Size, Mode: entry.Mode, ModTime: time.Unix(entry.ModTime, 0)})
	}
	return entries, nil
}

func (c *Client) NginxSiteApply(ctx context.Context, action contract.NginxSiteAction, site contract.NginxSiteSpec) error {
	protoAction := agentv1.NginxSiteAction_NGINX_SITE_ACTION_UNSPECIFIED
	switch action {
	case contract.NginxSiteUpsert:
		protoAction = agentv1.NginxSiteAction_NGINX_SITE_ACTION_UPSERT
	case contract.NginxSiteDelete:
		protoAction = agentv1.NginxSiteAction_NGINX_SITE_ACTION_DELETE
	case contract.NginxSiteEnable:
		protoAction = agentv1.NginxSiteAction_NGINX_SITE_ACTION_ENABLE
	case contract.NginxSiteDisable:
		protoAction = agentv1.NginxSiteAction_NGINX_SITE_ACTION_DISABLE
	default:
		return fmt.Errorf("agent client: unsupported nginx site action %q", action)
	}
	_, err := c.stub.NginxSiteApply(ctx, &agentv1.NginxSiteApplyRequest{
		Action:     protoAction,
		Domain:     site.Domain,
		RootPath:   site.RootPath,
		PhpVersion: site.PHPVersion,
		SslEnabled: site.SSLEnabled,
		SslCert:    site.SSLCert,
		SslKey:     site.SSLKey,
	})
	return err
}

func (c *Client) FirewallAllow(ctx context.Context, port int, protocol string) error {
	_, err := c.stub.FirewallRule(ctx, &agentv1.FirewallRuleRequest{Port: int32(port), Protocol: protocol, Action: agentv1.FirewallAction_FIREWALL_ACTION_ALLOW})
	return err
}
func (c *Client) FirewallDeny(ctx context.Context, port int, protocol string) error {
	_, err := c.stub.FirewallRule(ctx, &agentv1.FirewallRuleRequest{Port: int32(port), Protocol: protocol, Action: agentv1.FirewallAction_FIREWALL_ACTION_DENY})
	return err
}
func (c *Client) FirewallList(ctx context.Context) ([]contract.FirewallRule, error) {
	result, err := c.stub.FirewallList(ctx, &agentv1.FirewallListRequest{})
	if err != nil {
		return nil, err
	}
	rules := make([]contract.FirewallRule, 0, len(result.Rules))
	for _, rule := range result.Rules {
		rules = append(rules, contract.FirewallRule{Port: int(rule.Port), Protocol: rule.Protocol, Action: rule.Action})
	}
	return rules, nil
}

func (c *Client) SystemStats(ctx context.Context) (*contract.SystemStats, error) {
	result, err := c.stub.SystemStats(ctx, &agentv1.SystemStatsRequest{})
	if err != nil {
		return nil, err
	}
	stats := &contract.SystemStats{
		CPU:         contract.CPUStats{UsagePercent: result.CpuUsagePercent, Cores: int(result.CpuCores)},
		Memory:      contract.MemoryStats{Total: result.MemoryTotal, Used: result.MemoryUsed, Free: result.MemoryFree},
		Swap:        contract.SwapStats{Total: result.SwapTotal, Used: result.SwapUsed, Free: result.SwapTotal - result.SwapUsed},
		Network:     contract.NetworkStats{BytesSent: result.NetBytesSent, BytesRecv: result.NetBytesRecv},
		LoadAverage: [3]float64{result.Load_1, result.Load_5, result.Load_15},
		Temperature: result.Temperature, Uptime: result.Uptime,
	}
	if stats.Memory.Total > 0 {
		stats.Memory.UsedPercent = float64(stats.Memory.Used) / float64(stats.Memory.Total) * 100
	}
	if stats.Swap.Total > 0 {
		stats.Swap.UsedPercent = float64(stats.Swap.Used) / float64(stats.Swap.Total) * 100
	}
	for _, disk := range result.Disk {
		stats.Disk = append(stats.Disk, contract.DiskStats{Mountpoint: disk.Mountpoint, Total: disk.Total, Used: disk.Used, Free: disk.Free, UsedPercent: disk.UsedPercent})
	}
	return stats, nil
}

var _ contract.AgentClient = (*Client)(nil)
