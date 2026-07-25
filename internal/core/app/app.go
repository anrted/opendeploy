// Package app implements the application bootstrapper for OpenDeploy Core.
//
// App is the root of the dependency injection graph. It wires together all
// infrastructure (DB, logger, config) and domain services, then hands the
// assembled server to the caller for starting.
//
// Lifecycle:
//  1. New(cfg) — create all dependencies
//  2. Bootstrap(ctx) — run DB migrations, seed admin, bootstrap modules
//  3. Server().Start() — block on HTTP
//  4. Shutdown(ctx) — graceful drain
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/anrted/opendeploy/internal/agentclient"
	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/anrted/opendeploy/internal/core/dashboard"
	"github.com/anrted/opendeploy/internal/core/module"
	"github.com/anrted/opendeploy/internal/core/server"
	"github.com/anrted/opendeploy/internal/core/service"
	"github.com/anrted/opendeploy/internal/core/settings"
	"github.com/anrted/opendeploy/internal/core/site"
	"github.com/anrted/opendeploy/internal/core/updater"
	"github.com/anrted/opendeploy/internal/platform/config"
	"github.com/anrted/opendeploy/internal/platform/database"
	"github.com/anrted/opendeploy/internal/platform/database/sqlite"
	"github.com/anrted/opendeploy/internal/platform/events"
	"github.com/anrted/opendeploy/internal/platform/logger"
	"github.com/anrted/opendeploy/internal/platform/websocket"
	"github.com/anrted/opendeploy/pkg/contract"
	"github.com/anrted/opendeploy/pkg/version"
)

// App is the fully wired OpenDeploy Core application.
type App struct {
	cfg    *config.Config
	db     *database.Database
	logger *slog.Logger
	srv    *server.Server
	bus    *events.MemoryBus
	hub    *websocket.Hub
	agent  *agentclient.Client

	// Domain services
	authSvc   *auth.Service
	moduleSvc *module.Service

	// Module registry
	registry *module.Registry
}

// New wires the entire application dependency graph.
// It does NOT perform any I/O — all I/O happens in Bootstrap.
func New(cfg *config.Config) (*App, error) {
	// ── Logger ────────────────────────────────────────────────────────────
	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.File)
	if err != nil {
		return nil, fmt.Errorf("app: init logger: %w", err)
	}
	slog.SetDefault(log)

	// ── JWT manager ───────────────────────────────────────────────────────
	// JWT secret generation happens in Bootstrap where it can be properly stored.
	// Validate that the secret is usable if already configured.
	if cfg.Auth.JWTSecret != "" {
		if _, err := auth.NewJWTManager(cfg.Auth.JWTSecret); err != nil {
			return nil, fmt.Errorf("app: validate jwt secret: %w", err)
		}
	}

	// ── Event bus ─────────────────────────────────────────────────────────
	bus := events.NewMemoryBus()

	// ── WebSocket hub ─────────────────────────────────────────────────────
	hub := websocket.NewHub(log)

	// ── Module registry ───────────────────────────────────────────────────
	registry := module.NewRegistry()

	return &App{
		cfg:      cfg,
		logger:   log,
		bus:      bus,
		hub:      hub,
		registry: registry,
		authSvc:  nil, // populated in Bootstrap
	}, nil
}

// Bootstrap performs all I/O-dependent initialisation:
//   - Opens SQLite and runs migrations
//   - Wires domain services with real repositories
//   - Seeds the default admin user if the DB is empty
//   - Bootstraps enabled modules
//   - Builds the HTTP server
func (a *App) Bootstrap(ctx context.Context) error {
	// ── Database ──────────────────────────────────────────────────────────
	a.logger.Info("app: opening database", "dsn", a.cfg.Database.DSN)
	db, err := sqlite.Open(a.cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("app: open database: %w", err)
	}
	a.db = db

	// ── Auth ──────────────────────────────────────────────────────────────
	jwtSecret := ensureJWTSecret(a.cfg)
	jwtMgr, err := auth.NewJWTManager(jwtSecret)
	if err != nil {
		return fmt.Errorf("app: init jwt: %w", err)
	}

	userRepo := auth.NewSQLiteUserRepository(db.DB)
	sessionRepo := auth.NewSQLiteSessionRepository(db.DB)
	authSvc := auth.NewService(
		userRepo, sessionRepo, jwtMgr,
		a.cfg.Auth.AccessTokenTTL,
		a.cfg.Auth.RefreshTokenTTL,
		a.logger,
	)

	// Seed default admin if no users exist.
	adminPass := os.Getenv("OD_ADMIN_PASSWORD")
	if err := authSvc.SeedAdminIfEmpty(ctx, "admin", adminPass); err != nil {
		return fmt.Errorf("app: seed admin (set OD_ADMIN_PASSWORD to at least 12 characters on first start): %w", err)
	}
	a.authSvc = authSvc

	// Agent connection. Core never executes privileged operations itself.
	agentAddress := a.cfg.Agent.Socket
	if strings.HasPrefix(agentAddress, "/") {
		agentAddress = "unix://" + agentAddress
	}
	agent, err := agentclient.Dial(agentAddress, a.cfg.Agent.Timeout, a.logger)
	if err != nil {
		_ = db.Close()
		a.db = nil
		return fmt.Errorf("app: connect to agent: %w", err)
	}
	a.agent = agent

	// ── Audit ─────────────────────────────────────────────────────────────
	auditSvc := audit.NewService(db.DB)

	// ── Modules ───────────────────────────────────────────────────────────
	moduleRepo := module.NewSQLiteRepository(db.DB)
	jobRepo := module.NewSQLiteJobRepository(db.DB)
	moduleSvc := module.NewService(
		a.registry, moduleRepo, jobRepo, a.bus, auditSvc, a.logger,
	)
	a.moduleSvc = moduleSvc

	// Bootstrap enabled modules.
	deps := contract.ModuleDeps{
		Agent:  agent,
		DB:     db.DB,
		Events: eventBusAdapter{a.bus},
		Logger: a.logger,
	}
	loader := module.NewLoader(a.registry, moduleRepo, a.logger)
	if err := loader.Bootstrap(ctx, deps); err != nil {
		return fmt.Errorf("app: bootstrap modules: %w", err)
	}

	// ── Dashboard ─────────────────────────────────────────────────────────
	dashboardSvc := dashboard.NewService(
		db.DB, agent, moduleRepo, auditSvc, a.bus, a.hub, a.logger,
	)
	// Start background stats poller (polls every 5s, stops when app shuts down).
	dashboardSvc.StartPoller(ctx, 5*time.Second)

	// ── Sites ─────────────────────────────────────────────────────────────
	siteRepo := site.NewSQLiteRepository(db.DB)
	siteSvc := site.NewService(siteRepo, auditSvc, agent, a.registry, a.logger)

	// ── Services ──────────────────────────────────────────────────────────
	svcRepo := service.NewSQLiteRepository(db.DB)
	svcSvc := service.NewSvcService(svcRepo, agent, a.logger)

	// ── Settings ──────────────────────────────────────────────────────────
	settingsSvc := settings.NewService(db.DB, a.logger)
	updateSvc := updater.NewService(version.Version, version.Commit, agent)

	// ── HTTP Server ───────────────────────────────────────────────────────
	authHandler := auth.NewHandler(authSvc)
	moduleHandler := module.NewHandler(moduleSvc)
	dashboardHandler := dashboard.NewHandler(dashboardSvc, a.hub, a.logger)
	siteHandler := site.NewHandler(siteSvc)
	svcHandler := service.NewHandler(svcSvc)
	settingsHandler := settings.NewHandler(settingsSvc, updateSvc)

	srv := server.New(server.Dependencies{
		Config:           a.cfg,
		AuthHandler:      authHandler,
		JWTManager:       jwtMgr,
		ModuleHandler:    moduleHandler,
		WSHub:            a.hub,
		DashboardHandler: dashboardHandler,
		SiteHandler:      siteHandler,
		ServiceHandler:   svcHandler,
		SettingsHandler:  settingsHandler,
	}, a.logger)
	a.srv = srv

	a.logger.Info("app: bootstrap complete")
	return nil
}

// Server returns the HTTP server. Must call Bootstrap first.
func (a *App) Server() *server.Server {
	return a.srv
}

// Shutdown drains active connections and closes the database.
func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("app: shutting down")
	if a.srv != nil {
		if err := a.srv.Shutdown(ctx); err != nil {
			return err
		}
	}
	var shutdownErrors []error
	if a.agent != nil {
		shutdownErrors = append(shutdownErrors, a.agent.Close())
	}
	if a.db != nil {
		shutdownErrors = append(shutdownErrors, a.db.Close())
	}
	return errors.Join(shutdownErrors...)
}

// RegisterModule adds a module to the registry before Bootstrap is called.
// Modules are registered in cmd/core/main.go via blank imports.
func (a *App) RegisterModule(m contract.Module) {
	a.registry.Register(m)
}

// ─── helpers ───────────────────────────────────────────────────────────────

// ensureJWTSecret returns the configured secret or auto-generates one.
// Auto-generated secrets are ephemeral (sessions invalidated on restart).
func ensureJWTSecret(cfg *config.Config) string {
	if cfg.Auth.JWTSecret != "" {
		return cfg.Auth.JWTSecret
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	secret := hex.EncodeToString(b)
	cfg.Auth.JWTSecret = secret
	slog.Warn("app: JWT secret not configured — using ephemeral secret (set OD_JWT_SECRET in production)")
	return secret
}

// eventBusAdapter adapts events.MemoryBus to contract.EventBus.
type eventBusAdapter struct {
	bus *events.MemoryBus
}

func (a eventBusAdapter) Publish(ctx context.Context, event contract.Event) error {
	return a.bus.Publish(ctx, eventWrapper{event})
}

func (a eventBusAdapter) Subscribe(eventType string, handler contract.EventHandler) contract.EventUnsubscribeFn {
	unsub := a.bus.Subscribe(eventType, func(ctx context.Context, e events.Event) error {
		return handler(ctx, e)
	})
	return contract.EventUnsubscribeFn(unsub)
}

// eventWrapper bridges contract.Event to events.Event.
type eventWrapper struct {
	contract.Event
}
