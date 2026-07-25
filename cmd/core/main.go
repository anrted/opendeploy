// main is the entry point for OpenDeploy Core.
//
// It performs the following steps:
//  1. Parse the --config flag.
//  2. Load configuration from YAML.
//  3. Register modules (blank imports trigger init() functions).
//  4. Bootstrap the application (DB, services, HTTP server).
//  5. Start the HTTP server.
//  6. Block until SIGTERM / SIGINT, then gracefully shut down.
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anrted/opendeploy/internal/core/app"
	"github.com/anrted/opendeploy/internal/platform/config"
	moduleApache "github.com/anrted/opendeploy/modules/apache"
	moduleCertbot "github.com/anrted/opendeploy/modules/certbot"
	moduleGit "github.com/anrted/opendeploy/modules/git"
	moduleMySQL "github.com/anrted/opendeploy/modules/mysql"
	moduleNginx "github.com/anrted/opendeploy/modules/nginx"
	moduleNodejs "github.com/anrted/opendeploy/modules/nodejs"
	modulePHP "github.com/anrted/opendeploy/modules/php"
	"github.com/anrted/opendeploy/pkg/version"
)

func main() {
	configPath := flag.String("config", "/etc/opendeploy/opendeploy.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		log.Println(version.Info())
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	// Register all built-in modules.
	// In a plugin system this would discover .so files; here it is explicit.
	registerModules(application)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := application.Bootstrap(ctx); err != nil {
		log.Fatalf("bootstrap: %v", err)
	}

	// Handle OS signals for graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server in a goroutine.
	serverErr := make(chan error, 1)
	go func() {
		if err := application.Server().Start(); err != nil {
			serverErr <- err
		}
	}()

	slog.Info("opendeploy core ready", "version", version.Version)

	select {
	case sig := <-quit:
		slog.Info("received signal, shutting down", "signal", sig)
	case err := <-serverErr:
		slog.Error("server error", "error", err)
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("opendeploy core stopped")
}

// registerModules adds all built-in modules to the application.
// This function is the compile-time module registry.
// To add a new module: implement contract.Module, then add it here.
func registerModules(a *app.App) {
	a.RegisterModule(moduleNginx.New())
	a.RegisterModule(moduleApache.New())
	a.RegisterModule(modulePHP.New())
	a.RegisterModule(moduleNodejs.New())
	a.RegisterModule(moduleGit.New())
	a.RegisterModule(moduleCertbot.New())
	a.RegisterModule(moduleMySQL.New())
}
