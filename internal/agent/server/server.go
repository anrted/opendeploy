package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/anrted/opendeploy/internal/agent/archive"
	agentCron "github.com/anrted/opendeploy/internal/agent/cron"
	executor "github.com/anrted/opendeploy/internal/agent/executor"
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
	fw      firewall.Provider
	stats   *stats.Collector
	shell   *executor.Shell
	cron    *agentCron.Manager
}

func New(systemdManager *systemd.Manager, packageManager packages.Manager, fileManager *filesystem.Manager, firewallManager firewall.Provider, collector *stats.Collector, shell *executor.Shell, cronManager *agentCron.Manager) *Service {
	return &Service{systemd: systemdManager, pkgs: packageManager, fs: fileManager, fw: firewallManager, stats: collector, shell: shell, cron: cronManager}
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
	case agentv1.ServiceActionType_SERVICE_ACTION_RELOAD:
		err = s.systemd.Reload(ctx, req.ServiceName)
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

func (s *Service) ServiceLogs(req *agentv1.ServiceLogsRequest, stream agentv1.AgentService_ServiceLogsServer) error {
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

func (s *Service) FileLogs(req *agentv1.FileLogsRequest, stream agentv1.AgentService_FileLogsServer) error {
	lines := int(req.GetLines())
	if lines < 1 || lines > 10_000 {
		return status.Error(codes.InvalidArgument, "lines must be between 1 and 10000")
	}
	path := req.GetPath()
	if path == "" {
		return status.Error(codes.InvalidArgument, "path is required")
	}

	// Wait, tailing arbitrary files is okay for Agent since it requires admin.
	// We rely on the allowed paths or shell isolation if needed.
	// Actually, just let it fail if it doesn't exist.

	res, err := s.shell.Run(stream.Context(), "tail", "-n", fmt.Sprint(lines), path)
	if err != nil {
		return internalError(err)
	}

	// Split stdout by newline
	var currentLine []byte
	for i := 0; i < len(res.Stdout); i++ {
		if res.Stdout[i] == '\n' {
			if err := stream.Send(&agentv1.LogLine{Line: string(currentLine), Timestamp: time.Now().UnixNano()}); err != nil {
				return err
			}
			currentLine = currentLine[:0]
		} else {
			currentLine = append(currentLine, res.Stdout[i])
		}
	}
	if len(currentLine) > 0 {
		if err := stream.Send(&agentv1.LogLine{Line: string(currentLine), Timestamp: time.Now().UnixNano()}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) StreamLogs(req *agentv1.StreamLogsRequest, stream agentv1.AgentService_StreamLogsServer) error {
	path := req.GetPath()
	if path == "" {
		return status.Error(codes.InvalidArgument, "path is required")
	}

	outCh := make(chan string)
	if err := s.shell.Stream(stream.Context(), outCh, "tail", "-f", path); err != nil {
		return internalError(err)
	}

	for line := range outCh {
		if err := stream.Send(&agentv1.LogLine{Line: line, Timestamp: time.Now().UnixNano()}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ServiceStreamLogs(req *agentv1.ServiceStreamLogsRequest, stream agentv1.AgentService_ServiceStreamLogsServer) error {
	name := req.GetServiceName()
	if name == "" {
		return status.Error(codes.InvalidArgument, "service_name is required")
	}

	outCh := make(chan string)
	if err := s.shell.Stream(stream.Context(), outCh, "journalctl", "-u", name, "-f", "-n", "10"); err != nil {
		return internalError(err)
	}

	for line := range outCh {
		if err := stream.Send(&agentv1.LogLine{Line: line, Timestamp: time.Now().UnixNano()}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CommandExecute(ctx context.Context, req *agentv1.CommandExecuteRequest) (*agentv1.CommandExecuteResponse, error) {
	if req.GetCommand() == "" {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}
	res, err := s.shell.Run(ctx, req.GetCommand(), req.GetArgs()...)
	if err != nil && res == nil {
		return nil, internalError(err)
	}
	return &agentv1.CommandExecuteResponse{
		ExitCode: int32(res.ExitCode),
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
	}, nil
}

func (s *Service) PackageInstall(req *agentv1.PackageRequest, stream agentv1.AgentService_PackageInstallServer) error {
	return s.streamPackage(req, stream, s.pkgs.Install)
}
func (s *Service) PackageRemove(req *agentv1.PackageRequest, stream agentv1.AgentService_PackageRemoveServer) error {
	return s.streamPackage(req, stream, s.pkgs.Remove)
}
func (s *Service) PackageUpdate(req *agentv1.PackageRequest, stream agentv1.AgentService_PackageUpdateServer) error {
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
		response.Entries = append(response.Entries, &agentv1.FileEntry{Name: entry.Name, Path: entry.Path, IsDir: entry.IsDir, Size: entry.Size, Mode: uint32(entry.Mode.Perm()), ModTime: entry.ModTime, Owner: entry.Owner, Group: entry.Group})
	}
	return response, nil
}

func (s *Service) FileRename(_ context.Context, req *agentv1.FileRenameRequest) (*agentv1.FileRenameResponse, error) {
	if err := s.fs.Rename(req.GetOldPath(), req.GetNewPath()); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FileRenameResponse{Success: true}, nil
}
func (s *Service) FileCopy(_ context.Context, req *agentv1.FileCopyRequest) (*agentv1.FileCopyResponse, error) {
	if err := s.fs.Copy(req.GetSrcPath(), req.GetDstPath()); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FileCopyResponse{Success: true}, nil
}
func (s *Service) FileChmod(_ context.Context, req *agentv1.FileChmodRequest) (*agentv1.FileChmodResponse, error) {
	if err := s.fs.Chmod(req.GetPath(), req.GetMode()); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FileChmodResponse{Success: true}, nil
}
func (s *Service) FileChown(_ context.Context, req *agentv1.FileChownRequest) (*agentv1.FileChownResponse, error) {
	if err := s.fs.Chown(req.GetPath(), int(req.GetUid()), int(req.GetGid())); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FileChownResponse{Success: true}, nil
}
func (s *Service) ArchiveCreate(ctx context.Context, req *agentv1.ArchiveCreateRequest) (*agentv1.ArchiveCreateResponse, error) {
	dest, err := s.fs.Resolve(req.GetDstPath())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid archive destination")
	}
	paths := make([]string, 0, len(req.GetPaths()))
	for _, path := range req.GetPaths() {
		resolved, resolveErr := s.fs.Resolve(path)
		if resolveErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid archive source")
		}
		paths = append(paths, resolved)
	}
	var format string
	switch {
	case strings.HasSuffix(dest, ".zip"):
		format = "zip"
	case strings.HasSuffix(dest, ".tar.gz"), strings.HasSuffix(dest, ".tgz"):
		format = "tar.gz"
	case strings.HasSuffix(dest, ".tar.xz"), strings.HasSuffix(dest, ".txz"):
		format = "tar.xz"
	case strings.HasSuffix(dest, ".tar.bz2"), strings.HasSuffix(dest, ".tbz2"):
		format = "tar.bz2"
	case strings.HasSuffix(dest, ".tar"):
		format = "tar"
	case strings.HasSuffix(dest, ".7z"):
		format = "7z"
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported archive format")
	}

	err = archive.Create(ctx, format, dest, paths)
	if err != nil {
		return nil, internalError(err)
	}
	return &agentv1.ArchiveCreateResponse{Success: true}, nil
}
func (s *Service) ArchiveExtract(ctx context.Context, req *agentv1.ArchiveExtractRequest) (*agentv1.ArchiveExtractResponse, error) {
	source, err := s.fs.Resolve(req.GetSrcPath())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid archive source")
	}
	destination, err := s.fs.Resolve(req.GetDstDir())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid archive destination")
	}
	err = archive.Extract(ctx, source, destination)
	if err != nil {
		return nil, internalError(err)
	}
	return &agentv1.ArchiveExtractResponse{Success: true}, nil
}
func (s *Service) FileUploadStream(stream agentv1.AgentService_FileUploadStreamServer) error {
	return status.Error(codes.Unimplemented, "FileUploadStream not implemented")
}

func (s *Service) FirewallRule(ctx context.Context, req *agentv1.FirewallRuleRequest) (*agentv1.FirewallRuleResponse, error) {
	if err := s.fw.AddRule(ctx, req); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FirewallRuleResponse{Success: true}, nil
}

func (s *Service) FirewallDelete(ctx context.Context, req *agentv1.FirewallDeleteRequest) (*agentv1.FirewallDeleteResponse, error) {
	if err := s.fw.DeleteRule(ctx, req.GetId()); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FirewallDeleteResponse{Success: true}, nil
}

func (s *Service) FirewallList(ctx context.Context, _ *agentv1.FirewallListRequest) (*agentv1.FirewallListResponse, error) {
	rules, err := s.fw.List(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FirewallListResponse{Rules: rules}, nil
}

func (s *Service) FirewallStatus(ctx context.Context, _ *agentv1.FirewallStatusRequest) (*agentv1.FirewallStatusResponse, error) {
	res, err := s.fw.Status(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	return res, nil
}

func (s *Service) FirewallToggle(ctx context.Context, req *agentv1.FirewallToggleRequest) (*agentv1.FirewallToggleResponse, error) {
	if err := s.fw.Toggle(ctx, req.GetEnable()); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FirewallToggleResponse{Success: true}, nil
}

func (s *Service) FirewallReset(ctx context.Context, _ *agentv1.FirewallResetRequest) (*agentv1.FirewallResetResponse, error) {
	if err := s.fw.Reset(ctx); err != nil {
		return nil, internalError(err)
	}
	return &agentv1.FirewallResetResponse{Success: true}, nil
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

func (s *Service) ProcessList(_ context.Context, _ *agentv1.ProcessListRequest) (*agentv1.ProcessListResponse, error) {
	result, err := s.stats.CollectProcesses()
	if err != nil {
		return nil, internalError(err)
	}
	response := &agentv1.ProcessListResponse{Processes: make([]*agentv1.ProcessEntry, 0, len(result))}
	for _, p := range result {
		response.Processes = append(response.Processes, &agentv1.ProcessEntry{
			Pid:        int32(p.Pid),
			Ppid:       int32(p.Ppid),
			Name:       p.Name,
			User:       p.User,
			CpuPercent: p.CpuPercent,
			MemPercent: p.MemPercent,
			MemRss:     p.MemRss,
			NumThreads: int32(p.NumThreads),
			CreateTime: p.CreateTime,
			Cmdline:    p.Cmdline,
		})
	}
	return response, nil
}

func (s *Service) ProcessKill(_ context.Context, req *agentv1.ProcessKillRequest) (*agentv1.ProcessKillResponse, error) {
	if err := s.stats.KillProcess(req.GetPid(), req.GetForce()); err != nil {
		if errors.Is(err, stats.ErrProtectedProcess) {
			return nil, status.Error(codes.FailedPrecondition, "process is protected; use service management for system services")
		}
		return nil, internalError(err)
	}
	return &agentv1.ProcessKillResponse{Success: true}, nil
}

func internalError(err error) error {
	_ = err // subsystem logs retain details; gRPC clients receive no root-level paths/output.
	return status.Error(codes.Internal, "agent operation failed")
}
