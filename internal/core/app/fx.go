package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/anrted/opendeploy/internal/agentclient"
	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/anrted/opendeploy/internal/core/dashboard"
	"github.com/anrted/opendeploy/internal/core/logs"
	"github.com/anrted/opendeploy/internal/core/module"
	"github.com/anrted/opendeploy/internal/core/remote"
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

	"go.uber.org/fx"
)

// Module is the Fx module for the OpenDeploy core application.
var Module = fx.Options(
	fx.Provide(
		// Configuration
		func(path string) (*config.Config, error) {
			return config.Load(path)
		},

		// Infrastructure
		provideLogger,
		provideDatabase,
		events.NewMemoryBus,
		websocket.NewHub,
		provideAgentClient,
		func(client *agentclient.Client) contract.AgentClient {
			return client
		},
		module.NewRegistry,

		// Provide *sql.DB for repositories
		func(db *database.Database) *sql.DB {
			return db.DB
		},

		// Auth
		provideJWTManager,
		auth.NewSQLiteUserRepository,
		auth.NewSQLiteSessionRepository,
		provideAuthService,

		// Services
		audit.NewService,
		module.NewSQLiteRepository,
		module.NewSQLiteJobRepository,
		provideModuleService,
		dashboard.NewService,
		site.NewSQLiteRepository,
		provideSiteService,
		service.NewSQLiteRepository,
		provideSvcService,
		settings.NewService,
		provideUpdaterService,

		// Handlers
		auth.NewHandler,
		module.NewHandler,
		dashboard.NewHandler,
		site.NewHandler,
		service.NewHandler,
		settings.NewHandler,
		logs.NewRepository,
		logs.NewService,
		logs.NewHandler,
		remote.NewRepository,
		remote.NewService,
		remote.NewHandler,

		// Server
		provideServer,
	),
	fx.Invoke(
		registerModules,
		seedAdmin,
		startBackgroundJobs,
		startRemoteServerMonitor,
		registerDomainSubscribers,
		bootstrapModules,
		reconcileSites,
		startServer,
	),
)

func registerModules(modules []contract.Module, registry *module.Registry) {
	for _, m := range modules {
		registry.Register(m)
	}
}

// provideLogger creates the slog logger based on config.
func provideLogger(cfg *config.Config) (*slog.Logger, error) {
	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.File)
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}
	slog.SetDefault(log)
	return log, nil
}

// provideDatabase opens the SQLite database.
func provideDatabase(lc fx.Lifecycle, cfg *config.Config, log *slog.Logger) (*database.Database, error) {
	log.Info("opening database", "dsn", cfg.Database.DSN)
	db, err := sqlite.Open(cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Inject DB into global logger so it can persist system logs
	logger.SetDB(db.DB)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			log.Info("closing database")
			return db.Close()
		},
	})
	return db, nil
}

// provideAgentClient connects to the external agent.
func provideAgentClient(lc fx.Lifecycle, cfg *config.Config, log *slog.Logger) (*agentclient.Client, error) {
	agentAddress := cfg.Agent.Socket
	if len(agentAddress) > 0 && agentAddress[0] == '/' {
		agentAddress = "unix://" + agentAddress
	}
	agent, err := agentclient.Dial(agentAddress, cfg.Agent.Timeout, log)
	if err != nil {
		return nil, fmt.Errorf("connect to agent: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			log.Info("closing agent connection")
			return agent.Close()
		},
	})
	return agent, nil
}

func ensureJWTSecret(cfg *config.Config) string {
	if cfg.Auth.JWTSecret != "" {
		return cfg.Auth.JWTSecret
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	secret := hex.EncodeToString(b)
	cfg.Auth.JWTSecret = secret
	slog.Warn("app: JWT secret not configured — using ephemeral secret")
	return secret
}

func provideJWTManager(cfg *config.Config) (*auth.JWTManager, error) {
	secret := ensureJWTSecret(cfg)
	return auth.NewJWTManager(secret)
}

func provideAuthService(userRepo auth.UserRepository, sessionRepo auth.SessionRepository, jwtMgr *auth.JWTManager, cfg *config.Config, log *slog.Logger, auditSvc *audit.Service) *auth.Service {
	return auth.NewService(userRepo, sessionRepo, jwtMgr, cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL, log, auditSvc)
}

func provideModuleService(registry *module.Registry, repo module.Repository, jobRepo module.JobRepository, bus *events.MemoryBus, auditSvc *audit.Service, updates *updater.Service, log *slog.Logger) *module.Service {
	service := module.NewService(registry, repo, jobRepo, bus, auditSvc, log)
	service.SetBackupGuard(updates)
	return service
}

func provideSiteService(repo site.Repository, auditSvc *audit.Service, agent *agentclient.Client, registry *module.Registry, bus *events.MemoryBus, updates *updater.Service, settingsSvc *settings.Service, log *slog.Logger) *site.Service {
	service := site.NewService(repo, auditSvc, agent, registry, bus, log)
	service.SetBackupGuard(updates)
	service.SetSettings(settingsSvc)
	return service
}

func provideSvcService(repo service.Repository, agent *agentclient.Client, log *slog.Logger) *service.SvcService {
	return service.NewSvcService(repo, agent, log)
}

func provideUpdaterService(agent *agentclient.Client) *updater.Service {
	return updater.NewService(version.Version, version.Commit, agent)
}

func provideServer(cfg *config.Config, authHandler *auth.Handler, jwtMgr *auth.JWTManager, moduleHandler *module.Handler, dashboardHandler *dashboard.Handler, siteHandler *site.Handler, svcHandler *service.Handler, settingsHandler *settings.Handler, logsHandler *logs.Handler, remoteHandler *remote.Handler, registry *module.Registry, hub *websocket.Hub, log *slog.Logger) *server.Server {
	return server.New(server.Dependencies{
		Config:           cfg,
		AuthHandler:      authHandler,
		JWTManager:       jwtMgr,
		ModuleHandler:    moduleHandler,
		WSHub:            hub,
		DashboardHandler: dashboardHandler,
		SiteHandler:      siteHandler,
		ServiceHandler:   svcHandler,
		SettingsHandler:  settingsHandler,
		LogsHandler:      logsHandler,
		RemoteHandler:    remoteHandler,
		ModuleRegistry:   registry,
	}, log)
}

func startRemoteServerMonitor(lc fx.Lifecycle, service *remote.Service, log *slog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						if err := service.MarkStale(ctx); err != nil {
							log.ErrorContext(ctx, "remote server status refresh failed", "error", err)
						}
					}
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error { cancel(); return nil },
	})
}

// ── Invokers ─────────────────────────────────────────────────────────────

func seedAdmin(authSvc *auth.Service, log *slog.Logger) error {
	adminPass := os.Getenv("OD_ADMIN_PASSWORD")
	if err := authSvc.SeedAdminIfEmpty(context.Background(), "admin", adminPass); err != nil {
		return fmt.Errorf("seed admin (set OD_ADMIN_PASSWORD to at least 12 characters on first start): %w", err)
	}
	return nil
}

func startBackgroundJobs(lc fx.Lifecycle, dashboardSvc *dashboard.Service, log *slog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			// Start background stats poller (polls every 5s)
			dashboardSvc.StartPoller(ctx, 5*time.Second)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

func registerDomainSubscribers(lc fx.Lifecycle, bus *events.MemoryBus, auditSvc *audit.Service, hub *websocket.Hub, log *slog.Logger) {
	var unsubscribers []events.UnsubscribeFn
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			unsubscribers = append(unsubscribers,
				site.RegisterAuditSubscriber(bus, auditSvc, log),
				dashboard.RegisterSiteLifecycleSubscriber(bus, hub),
			)
			return nil
		},
		OnStop: func(context.Context) error {
			for _, unsubscribe := range unsubscribers {
				unsubscribe()
			}
			return nil
		},
	})
}

func bootstrapModules(lc fx.Lifecycle, registry *module.Registry, repo module.Repository, moduleService *module.Service, agent *agentclient.Client, db *database.Database, bus *events.MemoryBus, log *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := moduleService.RecoverInterruptedJobs(ctx); err != nil {
				return err
			}
			loader := module.NewLoader(registry, repo, log)
			deps := contract.ModuleDeps{
				Agent:  agent,
				DB:     db.DB,
				Events: eventBusAdapter{bus},
				Logger: log,
				Tasks:  moduleService,
			}
			return loader.Bootstrap(ctx, deps)
		},
	})
}

func reconcileSites(lc fx.Lifecycle, siteService *site.Service, log *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := siteService.ReconcileActive(ctx); err != nil {
				log.ErrorContext(ctx, "site reconciliation completed with errors", "error", err)
			}
			return nil
		},
	})
}

func startServer(lc fx.Lifecycle, srv *server.Server, log *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err := srv.Start(); err != nil {
					log.Error("server error", "error", err)
				}
			}()
			log.Info("server started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
}

// ─── helpers ───────────────────────────────────────────────────────────────

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

type eventWrapper struct {
	contract.Event
}
