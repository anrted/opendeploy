# OpenDeploy roadmap

This roadmap reflects the repository audit of 2026-07-27. Percentages and
priorities are recorded in [AUDIT.md](AUDIT.md). Dates are intentionally not
promised; exit criteria determine release readiness.

## Current version: 0.1.0-alpha

Implemented foundations:

- Core/Agent privilege separation over a local gRPC socket.
- Embedded Vue SPA, authentication/RBAC, SQLite repositories and audit records.
- Dashboard metrics/processes, module lifecycle, sites/files, services/logs,
  firewall and GitHub update status.
- Ubuntu-oriented installer/uninstaller and Linux CI.

Known limits: early-alpha security posture, sparse integration coverage, no
complete users/certificates/databases product areas and no persistent task queue.

## v0.8 — secure the foundation

### Security and architecture

- Enforce granular RBAC on every generic and module-registered route.
- Replace generic destructive Agent commands with typed operations and operand validation.
- Harden filesystem symlink containment and archive extraction.
- Define trusted-proxy handling, request limits and streaming backpressure.

### Quality and UX

- Add complete route authorization tests and module/Agent integration tests.
- Add permission-aware navigation/actions and consistent loading/error/empty states.
- Split the oversized site, module-manager and Nginx implementations (completed
  in the 2026-07-27 architecture pass); split Agent-client and statistics
  services in the next bounded refactoring pass.

### Architecture follow-up

The architecture pass also reduced `FileManager/index.vue` to a coordinator and
introduced dedicated child components and composables. The remaining
hand-written files above the 350-line guideline are tracked for focused,
behavior-preserving work:

- `pkg/contract/module.go`: separate capability contracts into cohesive files.
- `internal/agentclient/client.go`: split typed client adapters by Agent domain.
- `internal/agent/server/server.go`: split gRPC handlers by service area.
- `internal/agent/stats/collector.go`: split collectors by metric family.
- `internal/core/service/service.go` and `internal/core/auth/service.go`: split
  query, lifecycle and token/session responsibilities.
- large module views (`FirewallView.vue`, `ModuleDetailsView.vue`,
  `SettingsView.vue`): extract independent panels and state composables.
- `modules/fail2ban/module.go`, Agent executor and filesystem manager: extract
  parsing/validation and operation-specific adapters where cohesion warrants it.

Generated protobuf bindings are excluded from the source-size guideline.
- Fix cross-platform developer compilation or formally enforce Linux-only build tags.

### Modules

- Stabilize Nginx, Firewall and Fail2ban as supported alpha modules.
- Document unsupported/skeleton module capabilities explicitly.

## v0.9 — operational beta

### Features

- Durable tasks with cancellation, retries, idempotency and restart recovery.
- User administration, password change/reset and optional MFA.
- Certificate inventory, issue, renew, revoke and expiry reporting.
- MySQL/PostgreSQL database and user management with safe query construction.
- Backup/restore for OpenDeploy state, sites and databases.

### Delivery

- OpenAPI document validated in CI.
- Browser end-to-end suite and supported Linux distribution matrix.
- Enforced dependency-vulnerability thresholds.
- Signed/checksummed artifacts, update verification and tested rollback.
- Operational metrics, audit search/export and retention controls.

## v1.0 — first stable release

Exit criteria:

- All P0 findings in `AUDIT.md` closed with regression tests.
- Supported distro installations pass clean-host and upgrade tests.
- Documented threat model, backup/restore drill and incident/recovery runbook.
- Stable API compatibility policy and migration policy.
- Performance/soak targets met for dashboard, logs, tasks and large file trees.
- Nginx, Firewall, Services, Sites/Files, Users, Certificates and one database
  engine meet documented production acceptance criteria.

UX work includes accessibility review, destructive-action safeguards, actionable
job errors and complete permission-aware behavior.

## v1.1 — deployment workflows

- Complete PHP version/pool management.
- Node.js runtime and per-site process management.
- Git deployment with deploy keys, hooks, atomic releases and rollback.
- Scheduled backups, notification channels and certificate alerts.
- Additional supported Linux releases based on CI evidence.

## v2.0 — multi-server platform

- Remote Agent enrollment and mutual TLS with key rotation.
- Multi-server inventory, orchestration and policy.
- Optional PostgreSQL control-plane storage and high-availability design.
- Signed third-party module distribution, sandboxing and compatibility contracts.
- Organization/tenant boundaries and delegated administration.

Roadmap items are directional until they have issue-level acceptance criteria.
