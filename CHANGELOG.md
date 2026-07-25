# Changelog

## Unreleased

### Added

- Domain validation tests for site hostnames, Linux document roots, and PHP
  version selectors.
- Atomic Agent filesystem writes using a same-directory temporary file,
  file synchronization, rename, and Unix directory synchronization.
- Typed `NginxSiteApply` gRPC operation with vhost rendering, `nginx -t`,
  reload, and automatic rollback.
- Compensating Site lifecycle operations keep SQLite and Nginx state aligned.
- Production Vue SPA embedded in the Core binary with Vue Router fallback.

### Security

- Site document roots are restricted to descendants of `/var/www` and `/srv`.
- Site hostnames now follow DNS label rules; underscores, empty labels, and
  leading or trailing hyphens are rejected.
- Dashboard WebSocket connections use Gorilla WebSocket's same-origin policy.
- Removed permissive wildcard CORS with credentials; the embedded UI uses the
  same origin as the API.
- Nginx paths and DNS names are independently validated again at the privileged
  Agent boundary to prevent configuration-directive injection and traversal.

All notable changes to OpenDeploy are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased] — MVP v1.0.0

### Added

**Infrastructure (Stage 1)**
- Go module setup with `go.mod`
- `internal/platform/config` — YAML configuration with env overrides
- `internal/platform/logger` — slog-based structured logging with context propagation
- `internal/platform/database/sqlite` — SQLite connection with WAL mode + migrations
- `internal/platform/database/migrations` — Embedded SQL migrations (001-003)
- `internal/platform/apperrors` — Typed application errors with HTTP status mapping
- `internal/platform/events` — In-process publish/subscribe event bus
- `internal/platform/websocket` — WebSocket hub with room-based broadcasting
- `pkg/version` — Build-time version information
- `Makefile` — Build, test, lint, install targets
- `proto/agent/v1/agent.proto` — gRPC contract definition

**Public Contracts (Stage 1)**
- `pkg/contract/module.go` — Module, AgentClient, EventBus, SystemStats interfaces

**Authentication (Stage 2)**
- `internal/core/auth/domain.go` — User, Session, Role, Permission entities with RBAC
- `internal/core/auth/repository.go` — UserRepository, SessionRepository interfaces
- `internal/core/auth/sqlite_repository.go` — SQLite implementations
- `internal/core/auth/jwt.go` — HS256 JWT manager with algorithm validation
- `internal/core/auth/service.go` — Login, logout, refresh with token rotation
- `internal/core/auth/handler.go` — HTTP endpoints: /auth/login, /logout, /refresh, /me
- `internal/core/audit/service.go` — Append-only audit log

**HTTP Server (Stage 2)**
- `internal/core/server/server.go` — HTTP server with graceful shutdown
- `internal/core/server/middleware/auth.go` — JWT validation middleware
- `internal/core/server/middleware/ratelimit.go` — Per-IP rate limiting + panic recovery
- `internal/core/server/middleware/logger.go` — Request logging middleware

**Module System (Stage 3)**
- `internal/core/module/domain.go` — Module Record, Job entities
- `internal/core/module/repository.go` — Repository, JobRepository interfaces
- `internal/core/module/sqlite_repository.go` — SQLite implementations
- `internal/core/module/registry.go` — Module Registry + Loader
- `internal/core/module/service.go` — Install/Uninstall/Enable/Disable/Restart (async Jobs)
- `internal/core/module/handler.go` — HTTP handlers for module management

**Application Bootstrapper**
- `internal/core/app/app.go` — DI graph root (New + Bootstrap pattern)
- `cmd/core/main.go` — Core binary with graceful SIGTERM handling
- `cmd/agent/main.go` — Agent binary
- `cmd/cli/main.go` — CLI binary

**System Agent (Stage 4)**
- `internal/agent/executor/shell.go` — Allowlist validator + shell executor (no sh -c)
- `internal/agent/systemd/manager.go` — systemd service management
- `internal/agent/packages/manager.go` — APT/DNF/YUM package manager with auto-detection
- `internal/agent/filesystem/manager.go` — File operations with path validation
- `internal/agent/firewall/ufw.go` — UFW firewall management
- `internal/agent/app/app.go` — Agent bootstrapper
- `internal/agentclient/client.go` — gRPC client (contract.AgentClient adapter)

**Built-in Modules (Stage 7)**
- `modules/nginx/module.go` — Nginx web server module
- `modules/php/module.go` — PHP-FPM module (multi-version: 8.1-8.4)
- `modules/nodejs/module.go` — Node.js module
- `modules/git/module.go` — Git module

**Configuration & Deployment**
- `configs/opendeploy.yaml` — Default configuration
- `deployments/systemd/opendeploy-core.service`
- `deployments/systemd/opendeploy-agent.service`
- `README.md`, `ARCHITECTURE.md`, `API.md`, `SECURITY.md`

### Planned

- Frontend (Vue 3 + TailwindCSS + Pinia) — Stage 8
- Sites management — Stage 6
- Services management — Stage 6
- Dashboard with live metrics — Stage 5
- Settings management — Stage 6
- gRPC generated code (`make proto`) — requires `protoc` on build machine
