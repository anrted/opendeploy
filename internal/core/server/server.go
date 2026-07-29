// Package server implements the OpenDeploy Core HTTP server.
//
// Responsibilities:
//   - Create and configure the chi router with the full middleware chain.
//   - Register all domain handler routes under /api/v1/.
//   - Serve the embedded Vue SPA for all non-API paths.
//   - Expose /health for load balancer probes.
//   - Provide graceful shutdown via Shutdown(ctx).
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	csrf "filippo.io/csrf/gorilla"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/anrted/opendeploy/internal/core/dashboard"
	"github.com/anrted/opendeploy/internal/core/logs"
	"github.com/anrted/opendeploy/internal/core/module"
	"github.com/anrted/opendeploy/internal/core/remote"
	coreMiddleware "github.com/anrted/opendeploy/internal/core/server/middleware"
	"github.com/anrted/opendeploy/internal/core/service"
	"github.com/anrted/opendeploy/internal/core/settings"
	"github.com/anrted/opendeploy/internal/core/site"
	"github.com/anrted/opendeploy/internal/core/webui"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/internal/platform/config"
	"github.com/anrted/opendeploy/internal/platform/websocket"
)

// Server wraps net/http.Server with additional lifecycle management.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// Dependencies groups all handler dependencies needed to build the router.
type Dependencies struct {
	Config           *config.Config
	AuthHandler      *auth.Handler
	JWTManager       *auth.JWTManager
	ModuleHandler    *module.Handler
	WSHub            *websocket.Hub
	DashboardHandler *dashboard.Handler
	SiteHandler      *site.Handler
	ServiceHandler   *service.Handler
	SettingsHandler  *settings.Handler
	LogsHandler      *logs.Handler
	RemoteHandler    *remote.Handler
	ModuleRegistry   *module.Registry
}

type chiRouterWrapper struct {
	chi.Router
	prefix             string
	readPermission     auth.Permission
	mutationPermission auth.Permission
}

func (w chiRouterWrapper) Get(pattern string, handlerFn func(interface{}, interface{})) {
	w.Router.With(coreMiddleware.RequirePermission(w.readPermission)).Get(w.prefix+pattern, func(rw http.ResponseWriter, r *http.Request) { handlerFn(rw, r) })
}
func (w chiRouterWrapper) Post(pattern string, handlerFn func(interface{}, interface{})) {
	w.Router.With(coreMiddleware.RequirePermission(w.mutationPermission)).Post(w.prefix+pattern, func(rw http.ResponseWriter, r *http.Request) { handlerFn(rw, r) })
}
func (w chiRouterWrapper) Put(pattern string, handlerFn func(interface{}, interface{})) {
	w.Router.With(coreMiddleware.RequirePermission(w.mutationPermission)).Put(w.prefix+pattern, func(rw http.ResponseWriter, r *http.Request) { handlerFn(rw, r) })
}
func (w chiRouterWrapper) Delete(pattern string, handlerFn func(interface{}, interface{})) {
	w.Router.With(coreMiddleware.RequirePermission(w.mutationPermission)).Delete(w.prefix+pattern, func(rw http.ResponseWriter, r *http.Request) { handlerFn(rw, r) })
}

// New constructs a Server with the full middleware chain and route tree.
func New(deps Dependencies, logger *slog.Logger) *Server {
	r := buildRouter(deps, logger)

	addr := deps.Config.Addr()
	srv := &http.Server{
		Addr:           addr,
		Handler:        r,
		ReadTimeout:    deps.Config.Server.ReadTimeout,
		WriteTimeout:   deps.Config.Server.WriteTimeout,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return &Server{httpServer: srv, logger: logger}
}

// Start begins accepting connections. It blocks until the server closes.
func (s *Server) Start() error {
	s.logger.Info("http server started", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: listen: %w", err)
	}
	return nil
}

// Shutdown gracefully drains active connections within the given timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("http server shutting down")
	return s.httpServer.Shutdown(ctx)
}

// ─── Router ────────────────────────────────────────────────────────────────

func buildRouter(deps Dependencies, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	// ── Global middleware ─────────────────────────────────────────────────
	r.Use(coreMiddleware.Recover)
	r.Use(middleware.RequestID)
	r.Use(coreMiddleware.Logger(logger))
	r.Use(limitRequestBody(2 << 20))

	if deps.Config.Security.RateLimit.Enabled {
		r.Use(coreMiddleware.RateLimit(deps.Config.Security.RateLimit.RequestsPerMinute))
	}

	// ── CSRF Protection ───────────────────────────────────────────────────
	// Protect all POST/PUT/DELETE requests. Provide the token via a header
	// X-CSRF-Token that the frontend can read from the initial request
	// (or via a dedicated endpoint).
	// We use the same JWT secret for CSRF for simplicity, or a random 32-byte key.
	// In production, Secure should be true (HTTPS only).
	csrfSecure := deps.Config.Addr() == ":443" || deps.Config.Addr() == ":8443"
	csrfMiddleware := csrf.Protect(
		[]byte(deps.Config.Auth.JWTSecret),
		csrf.Secure(csrfSecure),
		csrf.Path("/"),
		csrf.CookieName("csrf_token"),      // the cookie sent to the client
		csrf.RequestHeader("X-CSRF-Token"), // the header the client must send back
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, cookieErr := r.Cookie("csrf_token")
			logger.Warn("csrf request rejected",
				"path", r.URL.Path,
				"reason", csrf.FailureReason(r),
				"has_cookie", cookieErr == nil,
				"has_header", r.Header.Get("X-CSRF-Token") != "",
			)
			apperrors.WriteHTTP(w, apperrors.Forbidden("CSRF token invalid or missing"))
		})),
	)
	protect := csrfMiddleware
	csrfMiddleware = func(next http.Handler) http.Handler {
		protected := protect(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if csrfExemptRequest(r) {
				r = csrf.UnsafeSkipCheck(r)
			}
			if !csrfSecure {
				r = csrf.PlaintextHTTPRequest(r)
			}
			protected.ServeHTTP(w, r)
		})
	}
	r.Use(csrfMiddleware)

	// ── Health check (unauthenticated) ────────────────────────────────────
	r.Get("/health", healthHandler)
	if deps.DashboardHandler != nil {
		r.Get("/api/v1/dashboard/ws", deps.DashboardHandler.WebSocket)
	}

	// ── API v1 ────────────────────────────────────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {
		// Public auth endpoints
		r.Get("/auth/csrf", deps.AuthHandler.CSRFToken)
		r.Post("/auth/login", deps.AuthHandler.Login)
		r.Post("/auth/refresh", deps.AuthHandler.Refresh)
		if deps.RemoteHandler != nil {
			r.Post("/agents/register", deps.RemoteHandler.Register)
			r.Post("/agents/heartbeat", deps.RemoteHandler.Heartbeat)
		}

		// Protected endpoints — require valid JWT
		r.Group(func(r chi.Router) {
			r.Use(coreMiddleware.Auth(deps.JWTManager))

			r.Post("/auth/logout", deps.AuthHandler.Logout)
			r.Get("/auth/me", deps.AuthHandler.Me)
			r.With(coreMiddleware.RequirePermission(auth.PermUserManage)).Get("/users", deps.AuthHandler.ListUsers)
			r.With(coreMiddleware.RequirePermission(auth.PermUserManage)).Post("/users", deps.AuthHandler.CreateUser)
			r.With(coreMiddleware.RequirePermission(auth.PermUserManage)).Put("/users/{id}", deps.AuthHandler.UpdateUser)
			r.With(coreMiddleware.RequirePermission(auth.PermUserManage)).Put("/users/{id}/password", deps.AuthHandler.ChangeUserPassword)
			r.With(coreMiddleware.RequirePermission(auth.PermUserManage)).Get("/users/{id}/audit", deps.AuthHandler.UserAudit)
			r.With(coreMiddleware.RequirePermission(auth.PermUserManage)).Post("/users/{id}/{action:block|unblock}", deps.AuthHandler.SetUserActive)
			r.With(coreMiddleware.RequirePermission(auth.PermUserManage)).Delete("/users/{id}", deps.AuthHandler.DeleteUser)

			// Module routes
			if deps.ModuleHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/modules", deps.ModuleHandler.List)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/modules/{id}", deps.ModuleHandler.Get)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/modules/{id}/status", deps.ModuleHandler.Status)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleInstall)).Post("/modules/{id}/install", deps.ModuleHandler.Install)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleUninstall)).Post("/modules/{id}/uninstall", deps.ModuleHandler.Uninstall)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleEnable)).Post("/modules/{id}/enable", deps.ModuleHandler.Enable)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleDisable)).Post("/modules/{id}/disable", deps.ModuleHandler.Disable)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/modules/{id}/restart", deps.ModuleHandler.Restart)

				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/modules/{id}/datagrid/{pageId}/schema", deps.ModuleHandler.HandleGetDataGridSchema)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/modules/{id}/datagrid/{pageId}/data", deps.ModuleHandler.HandleGetDataGridData)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/modules/{id}/datagrid/{pageId}/action/{actionId}", deps.ModuleHandler.HandleExecuteDataGridAction)

				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/modules/{id}/settings", deps.ModuleHandler.HandleSaveSettings)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/modules/{id}/presets", deps.ModuleHandler.ProtectionPresets)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Post("/modules/{id}/presets/{presetId}/preview", deps.ModuleHandler.PreviewProtectionPreset)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Put("/modules/{id}/presets/{presetId}", deps.ModuleHandler.SaveProtectionPreset)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/modules/{id}/presets/{presetId}/reset", deps.ModuleHandler.ResetProtectionPreset)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/modules/{id}/presets/{presetId}/toggle", deps.ModuleHandler.ToggleProtectionPreset)

				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/modules/{id}/logs/{logId}/read", deps.ModuleHandler.HandleReadLog)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/modules/{id}/logs/{logId}/clear", deps.ModuleHandler.HandleClearLog)

				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/modules/{id}/actions/{actionId}", deps.ModuleHandler.ExecuteAction)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/jobs/{id}", deps.ModuleHandler.GetJob)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/tasks", deps.ModuleHandler.ListJobs)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/tasks/{id}/cancel", deps.ModuleHandler.CancelJob)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/tasks/{id}/retry", deps.ModuleHandler.RetryJob)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Delete("/tasks/{id}", deps.ModuleHandler.DeleteJob)
			}

			// Register custom routes for each module
			if deps.ModuleRegistry != nil {
				for _, m := range deps.ModuleRegistry.All() {
					m.RegisterRoutes(chiRouterWrapper{
						Router:             r,
						prefix:             "/modules/" + m.ID(),
						readPermission:     auth.PermModuleView,
						mutationPermission: auth.PermModuleConfigure,
					})
				}
			}

			// Dashboard routes
			if deps.DashboardHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermDashboardView)).Get("/dashboard", deps.DashboardHandler.Overview)
				r.With(coreMiddleware.RequirePermission(auth.PermDashboardView)).Get("/dashboard/snapshots", deps.DashboardHandler.Snapshots)
				r.With(coreMiddleware.RequirePermission(auth.PermDashboardView)).Post("/dashboard/ws-ticket", deps.DashboardHandler.IssueWebSocketTicket)

				// System processes
				r.With(coreMiddleware.RequirePermission(auth.PermDashboardView)).Get("/system/processes", deps.DashboardHandler.ListProcesses)
				r.With(coreMiddleware.RequirePermission(auth.PermProcessManage)).Post("/system/processes/{pid}/kill", deps.DashboardHandler.KillProcess)
			}

			// Site routes
			if deps.SiteHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermSiteView)).Get("/sites", deps.SiteHandler.List)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteCreate)).Post("/sites", deps.SiteHandler.Create)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteView)).Get("/sites/{id}", deps.SiteHandler.Get)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Put("/sites/{id}", deps.SiteHandler.Update)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteDelete)).Delete("/sites/{id}", deps.SiteHandler.Delete)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Post("/sites/{id}/enable", deps.SiteHandler.Enable)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Post("/sites/{id}/disable", deps.SiteHandler.Disable)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteView)).Get("/sites/{id}/files", deps.SiteHandler.ListFiles)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteView)).Get("/sites/{id}/file", deps.SiteHandler.ReadFile)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Post("/sites/{id}/file", deps.SiteHandler.WriteFile)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Delete("/sites/{id}/file", deps.SiteHandler.DeleteFile)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Post("/sites/{id}/directory", deps.SiteHandler.CreateDirectory)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Post("/sites/{id}/files/batch", deps.SiteHandler.BatchOperations)
			}

			if deps.RemoteHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermServerView)).Get("/servers", deps.RemoteHandler.List)
				r.With(coreMiddleware.RequirePermission(auth.PermServerManage)).Post("/servers", deps.RemoteHandler.Create)
				r.With(coreMiddleware.RequirePermission(auth.PermServerView)).Get("/servers/{id}", deps.RemoteHandler.Get)
				r.With(coreMiddleware.RequirePermission(auth.PermServerManage)).Post("/servers/{id}/enrollment", deps.RemoteHandler.ReissueEnrollment)
				r.With(coreMiddleware.RequirePermission(auth.PermServerManage)).Delete("/servers/{id}", deps.RemoteHandler.Delete)
				r.With(coreMiddleware.RequirePermission(auth.PermServerManage)).Post("/servers/{id}/actions/{action}", deps.RemoteHandler.Action)
				r.With(coreMiddleware.RequirePermission(auth.PermServerView)).Get("/servers/{id}/events", deps.RemoteHandler.Events)
				r.With(coreMiddleware.RequirePermission(auth.PermServerView)).Get("/servers/{id}/heartbeats", deps.RemoteHandler.Heartbeats)
				r.With(coreMiddleware.RequirePermission(auth.PermServerView)).Get("/servers/{id}/tasks", deps.RemoteHandler.Tasks)
			}

			// Service routes
			if deps.ServiceHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermServiceView)).Get("/services", deps.ServiceHandler.List)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceManage)).Post("/services", deps.ServiceHandler.Add)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceManage)).Post("/services/{id}/start", deps.ServiceHandler.Start)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceManage)).Post("/services/{id}/stop", deps.ServiceHandler.Stop)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceManage)).Post("/services/{id}/restart", deps.ServiceHandler.Restart)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceView)).Get("/services/{id}/logs", deps.ServiceHandler.Logs)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceView)).Get("/services/{id}/logs/stream", deps.ServiceHandler.StreamLogs)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceManage)).Delete("/services/{id}", deps.ServiceHandler.Remove)
			}

			// Settings routes
			if deps.SettingsHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsView)).Get("/settings", deps.SettingsHandler.List)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsView)).Get("/settings/specs", deps.SettingsHandler.Specs)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsUpdate)).Put("/settings", deps.SettingsHandler.Update)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsView)).Get("/updates", deps.SettingsHandler.UpdateStatus)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsView)).Get("/updates/history", deps.SettingsHandler.UpdateHistory)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsSecurity)).Post("/updates/apply", deps.SettingsHandler.ApplyUpdate)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsSecurity)).Post("/updates/rollback", deps.SettingsHandler.RollbackUpdate)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsView)).Get("/backups/history", deps.SettingsHandler.BackupHistory)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsSecurity)).Post("/backups", deps.SettingsHandler.CreateBackup)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsSecurity)).Post("/backups/restore", deps.SettingsHandler.RestoreBackup)
			}

			// Logs routes
			if deps.LogsHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermDashboardView)).Mount("/logs", deps.LogsHandler.Routes())
			}
		})
	})

	// ── SPA catch-all: serve index.html for all non-API routes ───────────
	// The Vue SPA handles its own routing via Vue Router.
	r.Handle("/*", webui.Handler())

	return r
}

func limitRequestBody(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") && r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// csrfExemptRequest skips cookie-based CSRF checks where authentication does
// not use ambient browser credentials. Login and refresh exchange JSON tokens,
// while protected API calls authenticate with an explicit Bearer header.
func csrfExemptRequest(r *http.Request) bool {
	switch r.URL.Path {
	case "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/agents/register", "/api/v1/agents/heartbeat":
		return true
	}

	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Authorization"))), "bearer ")
}

// ─── Built-in handlers ─────────────────────────────────────────────────────

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","service":"opendeploy-core"}`))
}
