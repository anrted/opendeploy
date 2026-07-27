# OpenDeploy audit report

Audit date: 2026-07-27  
Audited revision: `1b96285`  
Scope: all tracked Go, Vue/JavaScript, SQL, protobuf, shell, CI and documentation files.

## Executive summary

OpenDeploy is a functioning early-alpha foundation, not a production-ready server
control panel. The repository contains a coherent Core/Agent split, an embedded
Vue SPA, SQLite persistence, typed gRPC operations, ten built-in modules and a
real installer/update path. The strongest implemented vertical slices are
authentication, dashboard telemetry, module lifecycle, systemd services, site
and file management, firewall management and transactional Nginx site changes.

Overall engineering score: **6.2/10**.  
Estimated feature readiness: **58%**.  
Production readiness after stage 1 hardening: **48%**.

Stage 1 remediation status and residual risks are documented in
[PRODUCTION_STAGE1.md](PRODUCTION_STAGE1.md). The findings below describe the
original audit baseline; module RBAC, command bounds, archive traversal, broad
filesystem roots, query JWT transport, panic recovery and UID/GID fallback have
been remediated. Symlink TOCTOU, cross-module transaction RPCs, durable
cancel/retry semantics and signed releases remain production gates.

The difference between feature and production readiness is deliberate: privileged
server administration requires stronger authorization, containment, rollback,
Linux integration tests and operational observability than an ordinary CRUD
application.

## Verification performed

| Check | Result |
|---|---|
| Repository inventory | 49 project Go packages, 10 modules, 8 primary SPA routes |
| Marker scan | No tracked TODO/FIXME/HACK markers outside generated/dependency files |
| Frontend unit tests | Pass: 1 file, 3 tests |
| Frontend lint | Pass |
| Frontend production build | Pass; about 123 KiB gzip total JS/CSS output |
| Go tests on Windows | Partially blocked by dependency-network sandbox; independent compile failure in `internal/agent/filesystem` (`syscall.Stat_t`) |
| CI inspection | Linux lint, race tests, frontend lint/tests, govulncheck and npm audit jobs exist |
| Documentation comparison | Multiple claims and version requirements were stale or overstated |

The local Go result is not reported as an application test failure where a module
could not be downloaded. The reproducible Windows compile error is a project
portability defect. Linux remains the declared runtime target, but Windows is
also used as a development environment and the file already contains Windows
build-tag variants elsewhere.

## Readiness by area

Percentages describe the current repository, not future intent.

| Area | Readiness | Assessment |
|---|---:|---|
| Core backend | 72% | Clear DI composition and service/repository separation; several large services and weak cross-cutting authorization consistency |
| System Agent | 63% | Broad typed gRPC surface and restricted socket; filesystem/archive/command policy needs hardening |
| CLI | 25% | Version and update-request handling only; no general administration client |
| Frontend | 68% | Eight usable routes, responsive shell, localization and confirmations; sparse tests and no permission-aware navigation |
| Authentication/RBAC | 67% | JWT, refresh rotation, Argon2id and permission middleware exist; module extension routes have authorization gaps |
| Dashboard/processes | 75% | Snapshot, live WebSocket metrics and process actions implemented; no historical retention or alerting |
| Modules | 62% | Registry, lifecycle jobs and UI are present; depth differs greatly per module |
| Sites/files | 73% | CRUD, batch file operations and transactional Nginx apply; no upload UX, backup workflow or complete certificate lifecycle |
| Services/logs | 70% | systemd list/actions and log streaming; limited filtering, retention and failure diagnostics |
| Firewall | 72% | UFW status/rules/toggle/reset and dedicated UI; privileged validation and integration coverage remain limited |
| Certificates | 35% | Certbot package/module foundation and ACME service code; no complete certificate inventory/renewal UI |
| Databases | 25% | MySQL module/database helpers and PostgreSQL package lifecycle; no production database-management surface |
| Users | 22% | Initial admin and roles exist; no user-management API or page |
| Settings/updater | 61% | Specs, persistence and GitHub update trigger; signed artifacts and robust rollback are absent |
| API | 66% | Versioned REST, JSON errors, validation and WebSocket/gRPC; no OpenAPI, pagination convention or batch contract standard |
| Installer/build/CI | 67% | Installer, uninstaller, Makefile and broad CI; distro matrix, release provenance and Windows developer build need work |
| Documentation | 70% | Core documents now describe the audited state; API remains manually maintained |

## Module review

| Module | Readiness | Implemented | Missing or risky |
|---|---:|---|---|
| Nginx | 78% | Package lifecycle, status, config tests, logs, sites, transactional apply | 554-line module, generic commands, incomplete integration/rollback coverage |
| Firewall | 75% | UFW rules, defaults, enable/disable/reset, UI | Linux integration tests, richer validation and lockout prevention |
| Fail2ban | 68% | Presets, jails/actions/settings/logs | Limited tests; command-oriented implementation |
| PHP | 48% | Package lifecycle and pool helpers | Full multi-version/repository flow, safe pool UI and integration tests |
| Node.js | 38% | Package lifecycle foundation | Version manager, per-site runtime, npm/deployment workflow |
| Git | 40% | Package lifecycle and basic actions | Credential handling, deploy keys, repository/project UI, rollback |
| Certbot | 38% | Package lifecycle and ACME foundation | Inventory, renewal state, revoke/replace, UI and end-to-end flow |
| MySQL | 28% | Package lifecycle and database helper | SQL construction is unsafe; credentials, CRUD UI and backup/restore |
| PostgreSQL | 20% | Package lifecycle foundation | Database/user management, repository, UI, backup/restore |
| Apache | 18% | Package lifecycle skeleton | Sites, configuration, logs, status details and UI |

## Confirmed findings

### Critical before production

1. **Module extension routes lack granular RBAC.** In
   `internal/core/server/server.go`, data-grid reads/actions, settings writes,
   log clearing, generic actions and routes registered by modules are inside the
   authentication group but do not apply `RequirePermission`. A viewer account
   can therefore reach mutating handlers if a module exposes them. Apply
   view/configure/manage permissions at routing boundaries and add table-driven
   authorization tests for every endpoint.

2. **The Agent command allowlist is argument-shape permissive.**
   `internal/agent/executor/shell.go` accepts every non-flag argument for allowed
   binaries, including destructive `rm`, ownership changes and package/service
   names. It prevents shell metacharacter interpretation in normal execution,
   but it is not a resource-level allowlist. Replace generic `CommandExecute`
   use with typed operations; validate package, service and path operands per
   command; remove `rm`, `chown`, `chmod` and Git mutation from the generic path.

3. **Archive extraction has no entry-level traversal/overwrite policy.**
   `internal/agent/archive/manager.go` delegates directly to tar/unzip/7z.
   Malicious entries can target paths outside the requested directory depending
   on tool behavior and archive format. Preflight entries, reject absolute and
   `..` paths and symlink escapes, extract into a temporary directory, then move
   validated output.

4. **Symlink containment is incomplete.**
   `internal/agent/filesystem/manager.go` performs lexical root checks but does
   not resolve existing symlinks for read/delete/copy/chmod/chown. A path below
   an allowed root can resolve outside it. Introduce operation-specific
   `openat2`/`O_NOFOLLOW`-style containment on Linux and never expose `/etc` or
   `/home` as unrestricted roots to general UI operations.

### High priority

5. **MySQL helper constructs SQL with string formatting.**
   `modules/mysql/db.go` interpolates database names, users and passwords.
   The current feature is not wired as production-ready, but activation would
   create SQL-injection and quoting defects. Use identifier validation plus
   parameterized values through a restricted administrative connection.

6. **CLI updater executes a shell script string.** `cmd/cli/main.go` calls
   `sh -c` for update content. Only a trusted root-written request should ever
   reach this path; still, replace string execution with a fixed executable and
   structured arguments, and verify artifact signatures/checksums.

7. **Authorization semantics are not mirrored in the UI.** Navigation and
   controls render for every authenticated role. Backend enforcement must remain
   authoritative, but the UI should hide or disable disallowed actions and
   explain `403` responses.

8. **Test density is too low for the privilege surface.** There are backend
   tests for selected auth, CSRF, tickets, site lifecycle, updater, filesystem
   and fail2ban behavior, but most modules and gRPC methods have no tests. The
   frontend has only three confirmation-store tests.

9. **CI does not enforce npm audit.** The workflow ends `npm audit` with
   `|| true`, making findings informational. Adopt severity thresholds and a
   documented exception process.

10. **No OpenAPI contract.** `API.md` is manually maintained and does not cover
    a uniform pagination/filtering/sorting model. Generate or validate an
    OpenAPI document in CI before declaring a stable API.

### Medium priority and technical debt

- `site.Service` (586 lines), Nginx module (554), Agent client (431), module
  service (398), stats collector (392) and core service manager (366) combine
  multiple responsibilities. Split orchestration, validation and adapters.
- Down migrations are empty. Roll-forward-only migration policy is acceptable
  only if documented and coupled with tested backups; otherwise implement safe
  down migrations.
- Rate limiting trusts `X-Real-IP` without a configured trusted-proxy boundary.
- Login/API documentation must not imply an interactive initial setup wizard;
  initial admin creation is environment-driven.
- UI version text was hardcoded as `v1.0.0` while the repository is
  `0.1.0-alpha`.
- There are no dedicated Certificates, Databases, Users or Logs routes. Their
  current functionality is module/detail or service-log based.
- No durable task queue exists. Module jobs are process-local, so restarts can
  lose execution state.
- WebSocket telemetry has no backpressure/retention SLO documented.
- Performance has no benchmark or load-test baseline. The frontend bundle is
  reasonable, but server CPU/memory and concurrent streaming limits are unknown.

## Completed functionality

- Three entry points: Core, Agent and CLI.
- Vue 3 SPA embedded into the Core production binary.
- JWT access authentication, refresh-token rotation, Argon2id password hashing,
  roles/permissions, CSRF middleware and request rate limiting.
- SQLite schema/migrations and repositories for implemented domains.
- Module registry, lifecycle jobs, status reconciliation and audit records.
- Dashboard snapshots, WebSocket tickets/live metrics and process listing/kill.
- systemd service actions and log read/streaming.
- Site CRUD, file browsing/editing/batch operations and Nginx apply with
  validation/compensation.
- UFW firewall management.
- Configuration manager, health endpoint and GitHub release update status.
- Ubuntu-oriented installer, uninstaller and systemd service definitions.

## Incomplete or placeholder functionality

- CLI administration beyond update/version behavior.
- User administration, invitation, password reset and MFA.
- Complete certificate inventory/renewal/revocation workflow.
- MySQL/PostgreSQL administration and database backup/restore.
- Apache site management.
- PHP/Node runtime/version/deployment management.
- Persistent task queue, restart recovery and task history UI.
- OpenAPI, pagination/filtering/sorting conventions and public compatibility policy.
- Multi-host management, remote Agent mTLS, signed modules and signed updates.
- Full audit-log UI/search/export and platform-wide log aggregation.

No explicit TODO/FIXME comments were found. The placeholders are behavioral
skeletons and roadmap-level omissions rather than marker comments.

## UI/UX review

| Page | Readiness | Notes |
|---|---:|---|
| Login | 75% | Functional; needs recovery/MFA and clearer deployment/TLS messaging |
| Dashboard | 78% | Useful live overview; needs error state, history and alert thresholds |
| Modules | 72% | Good catalog/lifecycle basis; status/job failure detail needs improvement |
| Module details | 68% | Flexible schema-driven UI; generic actions need permission metadata |
| Sites/files | 72% | Broad MVP workflow; needs upload, backups, conflict handling and safer destructive affordances |
| Services/logs | 70% | Core controls and streaming; needs search, download, retention and reconnect feedback |
| Processes | 68% | Listing and kill; needs permission-aware controls and stronger confirmation/context |
| Firewall | 73% | Dedicated rule UI; needs anti-lockout checks and clearer IPv4/IPv6/default-policy semantics |
| Settings/updates | 65% | Useful forms/update status; security-sensitive fields and update provenance need clearer treatment |

Missing standalone pages requested by product scope: Certificates, Databases,
Users, Audit Logs and System details. Current features do not justify presenting
those sections as complete.

## Performance assessment

- The production SPA build succeeds and code-splits routes. Largest emitted
  chunk is about 197 KiB raw/72 KiB gzip; no immediate bundle blocker exists.
- Dashboard polling/WebSocket behavior needs a documented concurrency budget and
  soak test. Process and system-stat collection can be expensive on large hosts.
- Module status reconciliation and external command calls need cancellation,
  concurrency limits and metrics.
- SQLite is reasonable for one host, but write contention, long tasks and audit
  retention require load tests and cleanup policies.
- Add benchmarks for snapshots, large directory listings, log streaming and
  repository queries; publish CPU/RSS and latency targets.

## Priority backlog

### P0: production blockers

- Close all module-route RBAC gaps and test the complete route matrix.
- Replace generic privileged command patterns with typed, resource-validated Agent RPCs.
- Harden filesystem and archive operations against symlink and archive traversal.
- Add Linux end-to-end tests for install, auth, sites, firewall, services and rollback.
- Sign/checksum release artifacts and make update rollback observable and tested.
- Establish supported distro/version matrix and backup/restore procedure.

### P1: v0.8/v0.9 quality

- Persistent task state, cancellation, retry/idempotency and recovery.
- User management, password change/reset and optional MFA.
- Certificate lifecycle and database-management MVPs.
- OpenAPI generation/validation and consistent collection query contracts.
- Permission-aware UI, accessibility pass and end-to-end browser suite.
- Metrics, audit search/export, structured operational events and SLOs.

### P2: v1.x

- PHP/Node/Git deployment workflows and tested backups.
- PostgreSQL option for Core storage where multi-host operation requires it.
- Remote Agents with mTLS, enrollment/rotation and tenant/host isolation.
- Signed third-party modules and compatibility policy.

## Release recommendation

Keep the current label **early alpha**. A limited, trusted-network test release
is reasonable after Linux CI is green. Do not expose OpenDeploy directly to the
Internet or claim production readiness until every P0 item has acceptance tests,
rollback procedures and release evidence.
