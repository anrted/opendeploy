// main is the entry point for the OpenDeploy Agent.
//
// The Agent runs as root and exposes a gRPC server on a Unix socket.
// Only the Core (opendeploy-core) is authorised to connect to this socket.
// The Agent never makes outbound network connections.
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	agentapp "github.com/anrted/opendeploy/internal/agent/app"
	agentCron "github.com/anrted/opendeploy/internal/agent/cron"
	"github.com/anrted/opendeploy/internal/platform/config"
	"github.com/anrted/opendeploy/pkg/version"
)

func main() {
	configPath := flag.String("config", "/etc/opendeploy/opendeploy.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	cronRun := flag.String("cron-run", "", "execute a managed Cron job and exit")
	flag.Parse()

	if *showVersion {
		log.Println(version.Info())
		os.Exit(0)
	}
	if *cronRun != "" {
		run, err := agentCron.NewManager().Run(context.Background(), *cronRun, "cron", "cron")
		if err != nil {
			log.Printf("cron job %s failed (exit %d): %v", *cronRun, run.ExitCode, err)
			os.Exit(1)
		}
		os.Exit(run.ExitCode)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	agent, err := agentapp.New(cfg)
	if err != nil {
		log.Fatalf("init agent: %v", err)
	}

	// Graceful shutdown on SIGTERM / SIGINT.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-quit
		slog.Info("agent: received signal, shutting down", "signal", sig)
		agent.Shutdown(context.Background())
	}()

	slog.Info("opendeploy agent starting", "version", version.Version)
	if err := agent.Start(); err != nil {
		slog.Error("agent error", "error", err)
		os.Exit(1)
	}
}
