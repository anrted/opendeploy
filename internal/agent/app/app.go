// Package agentapp bootstraps the OpenDeploy Agent.
package agentapp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"google.golang.org/grpc"

	"github.com/anrted/opendeploy/internal/agent/executor"
	"github.com/anrted/opendeploy/internal/agent/filesystem"
	"github.com/anrted/opendeploy/internal/agent/firewall"
	"github.com/anrted/opendeploy/internal/agent/packages"
	agentServer "github.com/anrted/opendeploy/internal/agent/server"
	"github.com/anrted/opendeploy/internal/agent/stats"
	agentSystemd "github.com/anrted/opendeploy/internal/agent/systemd"
	"github.com/anrted/opendeploy/internal/platform/config"
	"github.com/anrted/opendeploy/internal/platform/logger"
	"github.com/anrted/opendeploy/internal/platform/recovery"
)

// Agent is the fully wired OpenDeploy Agent.
type Agent struct {
	cfg        *config.Config
	logger     *slog.Logger
	grpcServer *grpc.Server

	// Sub-systems
	shell   *executor.Shell
	systemd *agentSystemd.Manager
	pkgs    packages.Manager
	fs      *filesystem.Manager
	fw      *firewall.UFWManager
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

	pkgMgr, err := packages.Detect(shell, log)
	if err != nil {
		log.Warn("agent: package manager detection failed — package operations will be unavailable", "error", err)
	}

	return &Agent{
		cfg:     cfg,
		logger:  log,
		shell:   shell,
		systemd: systemdMgr,
		pkgs:    pkgMgr,
		fs:      fsMgr,
		fw:      fwMgr,
	}, nil
}

// Start launches the gRPC server on the configured Unix socket.
func (a *Agent) Start() error {
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
	agentServer.New(a.systemd, a.pkgs, a.fs, a.fw, stats.NewCollector(), a.shell).Register(a.grpcServer)

	a.logger.Info("agent: gRPC server started", "socket", socketPath)
	return a.grpcServer.Serve(lis)
}

// Shutdown gracefully stops the gRPC server.
func (a *Agent) Shutdown(_ context.Context) {
	if a.grpcServer != nil {
		a.logger.Info("agent: shutting down gRPC server")
		a.grpcServer.GracefulStop()
	}
}

func removeSocket(path string) {
	os.Remove(path) //nolint:errcheck // ignore "not found" errors
}
