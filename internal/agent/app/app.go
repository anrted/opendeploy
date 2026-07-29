// Package agentapp bootstraps the OpenDeploy Agent.
package agentapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"

	"github.com/anrted/opendeploy/internal/agent/archive"
	agentCron "github.com/anrted/opendeploy/internal/agent/cron"
	"github.com/anrted/opendeploy/internal/agent/executor"
	"github.com/anrted/opendeploy/internal/agent/filesystem"
	"github.com/anrted/opendeploy/internal/agent/firewall"
	"github.com/anrted/opendeploy/internal/agent/packages"
	agentRemote "github.com/anrted/opendeploy/internal/agent/remote"
	agentServer "github.com/anrted/opendeploy/internal/agent/server"
	"github.com/anrted/opendeploy/internal/agent/stats"
	agentSystemd "github.com/anrted/opendeploy/internal/agent/systemd"
	"github.com/anrted/opendeploy/internal/platform/config"
	"github.com/anrted/opendeploy/internal/platform/logger"
	"github.com/anrted/opendeploy/internal/platform/recovery"
	"github.com/anrted/opendeploy/pkg/contract"
	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
)

// Agent is the fully wired OpenDeploy Agent.
type Agent struct {
	cfg        *config.Config
	logger     *slog.Logger
	grpcServer *grpc.Server
	remote     *agentRemote.Client
	stream     *agentRemote.StreamClient
	cancel     context.CancelFunc

	// Sub-systems
	shell   *executor.Shell
	systemd *agentSystemd.Manager
	pkgs    packages.Manager
	fs      *filesystem.Manager
	fw      *firewall.UFWManager
	cron    *agentCron.Manager
	stats   *stats.Collector
}

// New wires the Agent dependency graph.
func New(cfg *config.Config) (*Agent, error) {
	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format, "")
	if err != nil {
		return nil, fmt.Errorf("agent: init logger: %w", err)
	}

	validator := executor.NewValidator()
	shell := executor.NewShell(validator, log)
	systemdMgr := agentSystemd.NewManager(shell, log)
	fsMgr := filesystem.NewManager()
	fwMgr := firewall.NewUFWManager(shell, log)
	cronMgr := agentCron.NewManager()

	pkgMgr, err := packages.Detect(shell, log)
	if err != nil {
		log.Warn("agent: package manager detection failed — package operations will be unavailable", "error", err)
	}

	agent := &Agent{
		cfg:     cfg,
		logger:  log,
		shell:   shell,
		systemd: systemdMgr,
		pkgs:    pkgMgr,
		fs:      fsMgr,
		fw:      fwMgr,
		cron:    cronMgr,
		stats:   stats.NewCollector(),
	}
	if cfg.Agent.ControlPlaneAddress != "" {
		streamClient, streamErr := agentRemote.NewStream(cfg.Agent, log, agent.executeStreamCommand, agent.executeStreamSubscription)
		if streamErr != nil {
			return nil, fmt.Errorf("agent: init control-plane stream: %w", streamErr)
		}
		agent.stream = streamClient
	} else if cfg.Agent.CoreURL != "" {
		remoteClient, remoteErr := agentRemote.New(cfg.Agent, log)
		if remoteErr != nil {
			return nil, fmt.Errorf("agent: init remote client: %w", remoteErr)
		}
		agent.remote = remoteClient
	}
	return agent, nil
}

func (a *Agent) executeStreamSubscription(ctx context.Context, kind string, payload []byte, emit func([]byte) error) error {
	var request struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(payload, &request); err != nil {
		return err
	}
	lines := make(chan string, 64)
	switch kind {
	case "service.logs":
		if err := a.shell.Stream(ctx, lines, "journalctl", "-u", request.Name, "-f", "-n", "10"); err != nil {
			return err
		}
	case "file.logs":
		if err := a.shell.Stream(ctx, lines, "tail", "-f", request.Path); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported subscription %q", kind)
	}
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return nil
			}
			if err := emit([]byte(line)); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Start launches the gRPC server on the configured Unix socket.
func (a *Agent) Start() error {
	if a.stream != nil {
		ctx, cancel := context.WithCancel(context.Background())
		a.cancel = cancel
		go a.stream.Run(ctx)
		a.logger.Info("agent: control-plane stream started", "address", a.cfg.Agent.ControlPlaneAddress)
	} else if a.remote != nil {
		ctx, cancel := context.WithCancel(context.Background())
		a.cancel = cancel
		go a.remote.Run(ctx)
		a.logger.Info("agent: remote heartbeat client started", "core_url", a.cfg.Agent.CoreURL)
	}
	socketPath := a.cfg.Agent.Socket

	if err := os.MkdirAll(filepath.Dir(socketPath), 0o750); err != nil {
		return fmt.Errorf("agent: create socket directory: %w", err)
	}
	// Remove stale socket file if it exists.
	removeSocket(socketPath)

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("agent: listen on %q: %w", socketPath, err)
	}
	if err := os.Chmod(socketPath, 0o660); err != nil {
		_ = lis.Close()
		return fmt.Errorf("agent: secure socket %q: %w", socketPath, err)
	}

	a.grpcServer = grpc.NewServer(
		grpc.MaxRecvMsgSize(8<<20),
		grpc.MaxSendMsgSize(8<<20),
		grpc.ChainUnaryInterceptor(recovery.GRPCUnaryInterceptor()),
		grpc.ChainStreamInterceptor(recovery.GRPCStreamInterceptor()),
	)
	agentServer.New(a.systemd, a.pkgs, a.fs, a.fw, stats.NewCollector(), a.shell, a.cron).Register(a.grpcServer)

	a.logger.Info("agent: gRPC server started", "socket", socketPath)
	return a.grpcServer.Serve(lis)
}

func (a *Agent) executeStreamCommand(ctx context.Context, kind string, payload []byte) ([]byte, error) {
	var request struct {
		Name    string   `json:"name"`
		Action  string   `json:"action"`
		Path    string   `json:"path"`
		OldPath string   `json:"old_path"`
		NewPath string   `json:"new_path"`
		SrcPath string   `json:"src_path"`
		DstPath string   `json:"dst_path"`
		DstDir  string   `json:"dst_dir"`
		ID      string   `json:"id"`
		Package string   `json:"package"`
		Command string   `json:"command"`
		Trigger string   `json:"trigger"`
		Actor   string   `json:"actor"`
		PID     int32    `json:"pid"`
		Force   bool     `json:"force"`
		Enable  bool     `json:"enable"`
		Lines   int      `json:"lines"`
		Limit   int      `json:"limit"`
		UID     int      `json:"uid"`
		GID     int      `json:"gid"`
		Mode    uint32   `json:"mode"`
		Content []byte   `json:"content"`
		Paths   []string `json:"paths"`
		Args    []string `json:"args"`
	}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode %s command: %w", kind, err)
		}
	}
	var result any
	var err error
	switch kind {
	case "system.stats":
		result, err = a.stats.Collect()
	case "process.list":
		result, err = a.stats.CollectProcesses()
	case "process.kill":
		err = a.stats.KillProcess(request.PID, request.Force)
		result = map[string]bool{"success": err == nil}
	case "service.status":
		result, err = a.systemd.Status(ctx, request.Name)
	case "service.action":
		switch request.Action {
		case "start":
			err = a.systemd.Start(ctx, request.Name)
		case "stop":
			err = a.systemd.Stop(ctx, request.Name)
		case "restart":
			err = a.systemd.Restart(ctx, request.Name)
		case "enable":
			err = a.systemd.Enable(ctx, request.Name)
		case "disable":
			err = a.systemd.Disable(ctx, request.Name)
		case "reload":
			err = a.systemd.Reload(ctx, request.Name)
		default:
			err = fmt.Errorf("unsupported service action %q", request.Action)
		}
		result = map[string]bool{"success": err == nil}
	case "service.logs":
		result, err = a.systemd.Logs(ctx, request.Name, request.Lines)
	case "file.read":
		result, err = a.fs.Read(request.Path)
	case "file.write":
		err = a.fs.Write(request.Path, request.Content, fs.FileMode(request.Mode))
	case "file.delete":
		err = a.fs.Delete(request.Path)
	case "file.rename":
		err = a.fs.Rename(request.OldPath, request.NewPath)
	case "file.copy":
		err = a.fs.Copy(request.SrcPath, request.DstPath)
	case "file.chmod":
		err = a.fs.Chmod(request.Path, request.Mode)
	case "file.chown":
		err = a.fs.Chown(request.Path, request.UID, request.GID)
	case "directory.create":
		err = a.fs.MkdirAll(request.Path, fs.FileMode(request.Mode))
	case "file.list", "directory.list":
		var entries []filesystem.FileEntry
		entries, err = a.fs.List(request.Path)
		if err == nil {
			files := make([]contract.FileInfo, 0, len(entries))
			for _, entry := range entries {
				files = append(files, contract.FileInfo{
					Name: entry.Name, Path: entry.Path, IsDir: entry.IsDir, Size: entry.Size,
					Mode: uint32(entry.Mode), ModTime: time.Unix(entry.ModTime, 0),
					Owner: entry.Owner, Group: entry.Group,
				})
			}
			result = files
		}
	case "file.logs":
		var content []byte
		content, err = a.fs.Read(request.Path)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			if request.Lines > 0 && len(lines) > request.Lines {
				lines = lines[len(lines)-request.Lines:]
			}
			result = lines
		}
	case "archive.create":
		err = archive.Create(ctx, archiveFormat(request.DstPath), request.DstPath, request.Paths)
	case "archive.extract":
		err = archive.Extract(ctx, request.SrcPath, request.DstDir)
	case "firewall.status":
		var status *agentv1.FirewallStatusResponse
		status, err = a.fw.Status(ctx)
		if err == nil {
			result = &contract.FirewallStatus{
				Active: status.Active, DefaultIncoming: status.DefaultIncoming,
				DefaultOutgoing: status.DefaultOutgoing, DefaultRouted: status.DefaultRouted,
				IPv6Enabled: status.Ipv6Enabled, Logging: status.Logging,
				RuleCount: int(status.RuleCount), ProfileName: status.ProfileName,
			}
		}
	case "firewall.list":
		var entries []*agentv1.FirewallEntry
		entries, err = a.fw.List(ctx)
		if err == nil {
			rules := make([]*contract.FirewallRule, 0, len(entries))
			for _, entry := range entries {
				rules = append(rules, firewallContractRule(entry))
			}
			result = rules
		}
	case "firewall.delete":
		err = a.fw.DeleteRule(ctx, request.ID)
	case "firewall.toggle":
		err = a.fw.Toggle(ctx, request.Enable)
	case "firewall.reset":
		err = a.fw.Reset(ctx)
	case "firewall.rule":
		var rule struct {
			ID, Port, Protocol, Action, Direction, Source, Destination, Comment string
			IPVersion                                                           string `json:"ip_version"`
		}
		if err = json.Unmarshal(payload, &rule); err == nil {
			action := agentv1.FirewallAction_FIREWALL_ACTION_ALLOW
			if rule.Action == "deny" {
				action = agentv1.FirewallAction_FIREWALL_ACTION_DENY
			}
			if rule.Action == "reject" {
				action = agentv1.FirewallAction_FIREWALL_ACTION_REJECT
			}
			direction := agentv1.FirewallDirection_FIREWALL_DIRECTION_IN
			if rule.Direction == "out" {
				direction = agentv1.FirewallDirection_FIREWALL_DIRECTION_OUT
			}
			if rule.Direction == "routed" {
				direction = agentv1.FirewallDirection_FIREWALL_DIRECTION_ROUTED
			}
			ipVersion := agentv1.FirewallIPVersion_FIREWALL_IP_VERSION_BOTH
			if rule.IPVersion == "ipv4" {
				ipVersion = agentv1.FirewallIPVersion_FIREWALL_IP_VERSION_V4
			}
			if rule.IPVersion == "ipv6" {
				ipVersion = agentv1.FirewallIPVersion_FIREWALL_IP_VERSION_V6
			}
			err = a.fw.AddRule(ctx, &agentv1.FirewallRuleRequest{
				Id: rule.ID, Port: rule.Port, Protocol: rule.Protocol, Action: action,
				Direction: direction, Source: rule.Source, Destination: rule.Destination,
				Comment: rule.Comment, IpVersion: ipVersion,
			})
		}
	case "cron.list":
		result, err = a.cron.List()
	case "cron.get":
		result, err = a.cron.Get(request.ID)
	case "cron.create", "cron.update", "cron.validate":
		var job agentCron.Job
		if err = json.Unmarshal(payload, &job); err == nil {
			switch kind {
			case "cron.create":
				result, err = a.cron.Create(job)
			case "cron.update":
				result, err = a.cron.Update(job)
			default:
				validation, validationErr := agentCron.ValidateJob(job)
				result = validation
				err = validationErr
			}
		}
	case "cron.delete":
		err = a.cron.Delete(request.ID)
	case "cron.enable":
		result, err = a.cron.SetEnabled(request.ID, true)
	case "cron.disable":
		result, err = a.cron.SetEnabled(request.ID, false)
	case "cron.run":
		result, err = a.cron.Run(ctx, request.ID, request.Trigger, request.Actor)
	case "cron.history":
		result, err = a.cron.History(request.ID, request.Limit)
	case "command.execute":
		commandResult, commandErr := a.shell.Run(ctx, request.Command, request.Args...)
		err = commandErr
		if commandResult != nil {
			result = map[string]any{"exit_code": commandResult.ExitCode, "stdout": commandResult.Stdout, "stderr": commandResult.Stderr}
		}
	case "package.status":
		if a.pkgs == nil {
			err = fmt.Errorf("package manager unavailable")
			break
		}
		installed, packageVersion, packageErr := a.pkgs.Installed(ctx, request.Package)
		err = packageErr
		result = map[string]any{"installed": installed, "version": packageVersion}
	case "package.install", "package.remove", "package.update":
		if a.pkgs == nil {
			err = fmt.Errorf("package manager unavailable")
			break
		}
		var output <-chan string
		switch kind {
		case "package.install":
			output, err = a.pkgs.Install(ctx, request.Package)
		case "package.remove":
			output, err = a.pkgs.Remove(ctx, request.Package)
		default:
			output, err = a.pkgs.Update(ctx, request.Package)
		}
		if err == nil {
			var lines []string
			for line := range output {
				lines = append(lines, line)
			}
			result = lines
		}
	case "ping":
		result = map[string]string{"version": "v1"}
	default:
		return nil, fmt.Errorf("unsupported control-plane command %q", kind)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func archiveFormat(path string) string {
	for _, suffix := range []string{".tar.gz", ".tar.xz", ".tar.bz2", ".zip", ".tar", ".7z"} {
		if strings.HasSuffix(strings.ToLower(path), suffix) {
			return strings.TrimPrefix(suffix, ".")
		}
	}
	return "tar.gz"
}

func firewallContractRule(rule *agentv1.FirewallEntry) *contract.FirewallRule {
	action := "allow"
	if rule.Action == agentv1.FirewallAction_FIREWALL_ACTION_DENY {
		action = "deny"
	}
	if rule.Action == agentv1.FirewallAction_FIREWALL_ACTION_REJECT {
		action = "reject"
	}
	direction := "in"
	if rule.Direction == agentv1.FirewallDirection_FIREWALL_DIRECTION_OUT {
		direction = "out"
	}
	if rule.Direction == agentv1.FirewallDirection_FIREWALL_DIRECTION_ROUTED {
		direction = "routed"
	}
	ipVersion := "both"
	if rule.IpVersion == agentv1.FirewallIPVersion_FIREWALL_IP_VERSION_V4 {
		ipVersion = "ipv4"
	}
	if rule.IpVersion == agentv1.FirewallIPVersion_FIREWALL_IP_VERSION_V6 {
		ipVersion = "ipv6"
	}
	return &contract.FirewallRule{
		ID: rule.Id, Port: rule.Port, Protocol: rule.Protocol, Action: action,
		Direction: direction, Source: rule.Source, Destination: rule.Destination,
		Comment: rule.Comment, IPVersion: ipVersion,
	}
}

// Shutdown gracefully stops the gRPC server.
func (a *Agent) Shutdown(_ context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
	if a.grpcServer != nil {
		a.logger.Info("agent: shutting down gRPC server")
		a.grpcServer.GracefulStop()
	}
}

func removeSocket(path string) {
	os.Remove(path) //nolint:errcheck // ignore "not found" errors
}
