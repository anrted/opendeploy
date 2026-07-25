package server

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/anrted/opendeploy/internal/agent/filesystem"
	"github.com/anrted/opendeploy/internal/agent/firewall"
	"github.com/anrted/opendeploy/internal/agent/packages"
	"github.com/anrted/opendeploy/internal/agent/stats"
	"github.com/anrted/opendeploy/internal/agent/systemd"
	"github.com/anrted/opendeploy/pkg/version"
	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
)

// Service implements the privileged Agent gRPC boundary. It contains no
// shell-building logic; every operation is delegated to a validated subsystem.
type Service struct {
	agentv1.UnimplementedAgentServiceServer
	systemd *systemd.Manager
	pkgs    packages.Manager
	fs      *filesystem.Manager
	fw      *firewall.UFWManager
	stats   *stats.Collector
}

func New(systemdManager *systemd.Manager, packageManager packages.Manager, fileManager *filesystem.Manager, firewallManager *firewall.UFWManager, collector *stats.Collector) *Service {
	return &Service{systemd: systemdManager, pkgs: packageManager, fs: fileManager, fw: firewallManager, stats: collector}
}


func (s *Service) Register(registrar grpc.ServiceRegistrar) {
	agentv1.RegisterAgentServiceServer(registrar, s)
}

func (s *Service) ServiceAction(ctx context.Context, req *agentv1.ServiceActionRequest) (*agentv1.ServiceActionResponse, error) {
	if req.GetServiceName() == "" {
		return nil, status.Error(codes.InvalidArgument, "service_name is required")
	}
	var err error
	switch req.GetAction() {
	case agentv1.ServiceActionType_SERVICE_ACTION_START:
		err = s.systemd.Start(ctx, req.ServiceName)
	case agentv1.ServiceActionType_SERVICE_ACTION_STOP:
		err = s.systemd.Stop(ctx, req.ServiceName)
	case agentv1.ServiceActionType_SERVICE_ACTION_RESTART:
		err = s.systemd.Restart(ctx, req.ServiceName)
	case agentv1.ServiceActionType_SERVICE_ACTION_ENABLE:
		err = s.systemd.Enable(ctx, req.ServiceName)
	case agentv1.ServiceActionType_SERVICE_ACTION_DISABLE:
		err = s.systemd.Disable(ctx, req.ServiceName)
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported service action")
	}
	if err != nil {
		return nil, internalError(err)
	}
	return &agentv1.ServiceActionResponse{Success: true}, nil
}

func (s *Service) ServiceStatus(ctx context.Context, req *agentv1.ServiceStatusRequest) (*agentv1.ServiceStatusResponse, error) {
	result, err := s.systemd.Status(ctx, req.GetServiceName())
	if err != nil {
		return nil, internalError(err)
	}
	return &agentv1.ServiceStatusResponse{Name: result.Name, Active: result.Active, Enabled: result.Enabled, SubState: result.SubState, Description: result.Description}, nil
}

func (s *Service) ServiceLogs(req *agentv1.ServiceLogsRequest, stream grpc.ServerStreamingServer[agentv1.LogLine]) error {
	lines := int(req.GetLines())
	if lines < 1 || lines > 10_000 {
		return status.Error(codes.InvalidArgument, "lines must be between 1 and 10000")
	}
	result, err := s.systemd.Logs(stream.Context(), req.GetServiceName(), lines)
	if err != nil {
		return internalError(err)
	}
	for _, line := range result {
		if err := stream.Send(&agentv1.LogLine{Line: line, Timestamp: time.Now().UnixNano()}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) PackageInstall(req *agentv1.PackageRequest, stream grpc.ServerStreamingServer[agentv1.PackageOutput]) error {
	return s.streamPackage(req, stream, s.pkgs.Install)
}
func (s *Service) PackageRemove(req *agentv1.PackageRequest, stream grpc.ServerStreamingServer[agentv1.PackageOutput]) error {
	return s.streamPackage(req, stream, s.pkgs.Remove)
}
func (s *Service) PackageUpdate(req *agentv1.PackageRequest, stream grpc.ServerStreamingServer[agentv1.PackageOutput]) error {
	return s.streamPackage(req, stream, s.pkgs.Update)
}

type packageStream interface {
	Context() context.Context
	Send(*agentv1.PackageOutput) error
}

func (s *Service) streamPackage(req *agentv1.PackageRequest, stream packageStream, operation func(context.Context, string) (<-chan string, error)) error {
	if s.pkgs == nil {
		return status.Error(codes.FailedPrecondition, "no supported package manager is available")
	}
	output, err := operation(stream.Context(), req.GetPackageName())
	if err != nil {
		return internalError(err)
	}
	for line := range output {
		if err := stream.Send(&agentv1.PackageOutput{Line: line}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) PackageStatus(ctx context.Context, req *agentv1.PackageRequest) (*agentv1.PackageStatusResponse, error) {
	if s.pkgs == nil {
		return nil, status.Error(codes.FailedPrecondition, "no supported package manager is available")
	}
	installed, packageVersion, err := s.pkgs.Installed(ctx, req.GetPackageName())
	if err != nil {
		return nil, internalError(err)
	}
	return &agentv1.PackageStatusResponse{Installed: installed, Version: packageVersion}, nil
}

func (s *Service) FileRead(_ context.Context, req *agentv1.FileReadRequest) (*agentv1.FileReadResponse, error) {
	content, err := s.fs.Read(req.GetPath())
	if err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FileReadResponse{Content: content}, nil
}
func (s *Service) FileWrite(_ context.Context, req *agentv1.FileWriteRequest) (*agentv1.FileWriteResponse, error) {
	if err := s.fs.Write(req.GetPath(), req.GetContent(), fs.FileMode(req.GetMode())); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FileWriteResponse{Success: true}, nil
}
func (s *Service) FileDelete(_ context.Context, req *agentv1.FileDeleteRequest) (*agentv1.FileDeleteResponse, error) {
	if err := s.fs.Delete(req.GetPath()); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FileDeleteResponse{Success: true}, nil
}
func (s *Service) DirCreate(_ context.Context, req *agentv1.DirCreateRequest) (*agentv1.DirCreateResponse, error) {
	if err := s.fs.MkdirAll(req.GetPath(), fs.FileMode(req.GetMode())); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.DirCreateResponse{Success: true}, nil
}
func (s *Service) DirList(_ context.Context, req *agentv1.DirListRequest) (*agentv1.DirListResponse, error) {
	entries, err := s.fs.List(req.GetPath())
	if err != nil {
		return nil, internalError(err)
	}
	response := &agentv1.DirListResponse{Entries: make([]*agentv1.FileEntry, 0, len(entries))}
	for _, entry := range entries {
		response.Entries = append(response.Entries, &agentv1.FileEntry{Name: entry.Name, Path: entry.Path, IsDir: entry.IsDir, Size: entry.Size, Mode: uint32(entry.Mode.Perm()), ModTime: entry.ModTime})
	}
	return response, nil
}

func (s *Service) FirewallRule(ctx context.Context, req *agentv1.FirewallRuleRequest) (*agentv1.FirewallRuleResponse, error) {
	var err error
	switch req.GetAction() {
	case agentv1.FirewallAction_FIREWALL_ACTION_ALLOW:
		err = s.fw.Allow(ctx, int(req.GetPort()), req.GetProtocol())
	case agentv1.FirewallAction_FIREWALL_ACTION_DENY:
		err = s.fw.Deny(ctx, int(req.GetPort()), req.GetProtocol())
	case agentv1.FirewallAction_FIREWALL_ACTION_DELETE:
		err = s.fw.Delete(ctx, int(req.GetPort()), req.GetProtocol())
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported firewall action")
	}
	if err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FirewallRuleResponse{Success: true}, nil
}
func (s *Service) FirewallList(ctx context.Context, _ *agentv1.FirewallListRequest) (*agentv1.FirewallListResponse, error) {
	rules, err := s.fw.List(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	response := &agentv1.FirewallListResponse{Rules: make([]*agentv1.FirewallEntry, 0, len(rules))}
	for _, rule := range rules {
		response.Rules = append(response.Rules, &agentv1.FirewallEntry{Port: int32(rule.Port), Protocol: rule.Protocol, Action: rule.Action})
	}
	return response, nil
}

func (s *Service) SystemStats(_ context.Context, _ *agentv1.SystemStatsRequest) (*agentv1.SystemStatsResponse, error) {
	result, err := s.stats.Collect()
	if err != nil {
		return nil, internalError(err)
	}
	response := &agentv1.SystemStatsResponse{
		CpuUsagePercent: result.CPU.UsagePercent, CpuCores: int32(result.CPU.Cores),
		MemoryTotal: result.Memory.Total, MemoryUsed: result.Memory.Used, MemoryFree: result.Memory.Free,
		SwapTotal: result.Swap.Total, SwapUsed: result.Swap.Used,
		NetBytesSent: result.Network.BytesSent, NetBytesRecv: result.Network.BytesRecv,
		Load_1: result.LoadAverage[0], Load_5: result.LoadAverage[1], Load_15: result.LoadAverage[2],
		Temperature: result.Temperature, Uptime: result.Uptime,
	}
	for _, disk := range result.Disk {
		response.Disk = append(response.Disk, &agentv1.DiskStatEntry{Mountpoint: disk.Mountpoint, Total: disk.Total, Used: disk.Used, Free: disk.Free, UsedPercent: disk.UsedPercent})
	}
	return response, nil
}

func (s *Service) Ping(context.Context, *agentv1.PingRequest) (*agentv1.PingResponse, error) {
	return &agentv1.PingResponse{Version: version.Version}, nil
}

func internalError(err error) error {
	return status.Error(codes.Internal, fmt.Sprintf("agent operation failed: %v", err))
}
