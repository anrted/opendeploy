# OpenDeploy Architecture

## Overview

OpenDeploy consists of three independent processes with strict privilege separation:

1. **Core** (`opendeploy-core`) — runs as an unprivileged user, handles HTTP API, authentication, module registry, and WebSocket
2. **Agent** (`opendeploy-agent`) — runs as root, handles ALL system operations through gRPC
3. **CLI** (`opendeploy`) — command-line tool for scripted management

## Design Principles

### Security Isolation

The Backend (Core) **never** executes shell commands or accesses system files directly. Every privileged operation is delegated to the Agent via gRPC over a Unix socket. This means:

- A vulnerability in the Core API cannot directly lead to root code execution
- The Core can run with minimal OS capabilities
- The Agent validates every request against an explicit allowlist

### Modular Architecture

The core knows only the `contract.Module` interface. Modules register themselves at startup, contributing routes, menu items, and settings. **Adding a new module requires zero changes to the core.**

```
pkg/contract/module.go  ← Only thing modules depend on
        │
        ├── modules/nginx/module.go   ← implements contract.Module
        ├── modules/php/module.go
        ├── modules/nodejs/module.go
        └── modules/git/module.go
```

### Dependency Injection

All components receive their dependencies via constructor injection. There are no global singletons (except `slog.Default()`). This enables:
- Easy unit testing with mock implementations
- Clear dependency graph
- No hidden coupling

## Directory Structure

```
opendeploy/
├── cmd/            # Binary entry points
│   ├── core/       # HTTP API server
│   ├── agent/      # gRPC system agent
│   └── cli/        # CLI tool
├── internal/       # Private implementation
│   ├── core/       # Core domain (auth, modules, sites, dashboard)
│   │   ├── app/    # DI bootstrapper
│   │   ├── auth/   # Authentication & RBAC
│   │   ├── audit/  # Audit log
│   │   ├── module/ # Module registry & lifecycle
│   │   └── server/ # HTTP server & middleware
│   ├── agent/      # Agent domain
│   │   ├── app/    # Agent bootstrapper
│   │   ├── executor/   # Shell command executor (allowlist)
│   │   ├── systemd/    # systemd management
│   │   ├── packages/   # APT/DNF package manager
│   │   ├── filesystem/ # File operations (path validation)
│   │   └── firewall/   # UFW management
│   ├── agentclient/    # gRPC client (Core→Agent)
│   └── platform/       # Infrastructure
│       ├── config/     # YAML configuration
│       ├── database/   # Database abstraction
│       ├── events/     # In-process event bus
│       ├── logger/     # Structured logging (slog)
│       └── websocket/  # WebSocket hub
├── modules/        # Built-in module implementations
├── pkg/            # Public packages
│   ├── contract/   # Module, AgentClient, EventBus interfaces
│   └── version/    # Build-time version info
├── proto/          # Protobuf definitions (Core ↔ Agent)
├── web/            # Vue 3 frontend (Vite + TailwindCSS)
├── configs/        # Default configuration files
└── deployments/    # systemd service units
```

## Request Flow Example: Install Nginx

```
Browser: POST /api/v1/modules/nginx/install
  │
  ├─ [Auth middleware] Validate JWT, check role=admin
  ├─ [RateLimit middleware] 60 req/min per IP
  │
  ▼ ModuleHandler.Install(w, r)
  ├─ Extract principal from context
  │
  ▼ ModuleService.Install(ctx, "nginx", userID, ip)
  ├─ Check module exists in Registry
  ├─ Check module not already installed
  ├─ Create Job {id: uuid, type: "install_module", state: "pending"}
  ├─ Update module state → "installing"
  ├─ Record audit log entry
  ├─ Return HTTP 202 Accepted {job_id: "..."}
  │
  └─ [Async goroutine]
       ├─ Job state → "running"
       ├─ NginxModule.Install(ctx)
       │     └─ AgentClient.PackageInstall(ctx, "nginx")
       │           └─ [gRPC → Unix Socket → Agent]
       │                 ├─ CommandValidator: is "apt-get install nginx" allowed? YES
       │                 └─ Shell.Run("apt-get", "install", "-y", "nginx")
       │                       └─ exec.Command (no shell, PATH restricted)
       │                             └─ streaming stdout → outCh → Job.output
       ├─ Module state → "installed"
       ├─ Job state → "success"
       └─ EventBus.Publish("module.installed") → WebSocket broadcast
```

## Architecture Decision Records

See [docs/adr/](docs/adr/) for documented architectural decisions.

### ADR-001: gRPC for Core↔Agent Communication

**Decision**: Use gRPC over Unix socket instead of REST, D-Bus, or direct subprocess calls.

**Rationale**:
- Type-safe contracts via protobuf
- Streaming support for long-running operations (apt install)
- Easy to switch to TCP+mTLS for remote agents in the future
- Better than D-Bus: no daemon dependency, simpler auth

### ADR-002: Compile-time Module Registration

**Decision**: Modules are registered via explicit function calls in `cmd/core/main.go`, not via `.so` plugins.

**Rationale**:
- Go plugin system is fragile (same compiler version required, no cross-compilation)
- Compile-time registration is safe, fast, and easy to understand
- Plugin loading is listed in ROADMAP.md for when genuinely needed

### ADR-003: SQLite for MVP

**Decision**: Use SQLite as the primary database for MVP.

**Rationale**:
- Zero external dependency
- Sufficient for single-server deployment
- WAL mode provides good concurrent read performance
- Repository pattern ensures easy migration to PostgreSQL later

### ADR-004: Transactional Nginx site application

**Decision**: Site configuration is applied by the privileged Agent through the
typed `NginxSiteApply` gRPC operation.

**Rationale**:
- Core never receives an arbitrary command execution primitive.
- The Agent independently validates every DNS name and filesystem path.
- Vhost files are replaced atomically and enabled through a managed symlink.
- `nginx -t` runs before reload.
- Validation or reload failure restores the previous file and symlink state.
- Core uses compensating operations if SQLite persistence fails.
