# Production preparation — stage 1 report

Completed: 2026-07-27  
Scope: security and architectural foundation only; no new user-facing feature
was added.

## Outcome

Stage 1 closes the directly exploitable authorization, command, filesystem and
archive issues identified by the initial audit and introduces bounded execution,
restart reconciliation, safe configuration transactions, panic containment and
OS-family detection. OpenDeploy remains pre-release: distribution integration,
operation-specific durable retries and Linux race/rollback tests remain release
gates.

## Remediations

### Module authorization

- **Problem:** data-grid actions, settings, log clearing, generic actions and
  module-registered routes were authenticated but lacked granular permission
  checks.
- **Risk:** a viewer could invoke mutations implemented by a module.
- **Fix:** all generic reads require `module:view`; mutations and custom
  POST/PUT/DELETE routes require `module:configure`.
- **Files:** `internal/core/server/server.go`,
  `internal/core/server/module_rbac_test.go`.
- **Tests:** viewer read, viewer mutation denial and operator mutation.

### JWT transport and HTTP boundaries

- **Problem:** normal authentication middleware accepted JWTs in query strings;
  request bodies and headers had no explicit application limits.
- **Risk:** tokens leak through logs/history/referrers; oversized requests cause
  memory/resource pressure.
- **Fix:** bearer header only, 2 MiB API body cap, 1 MiB header cap and
  RemoteAddr-only rate-limit identity unless trusted-proxy support is explicitly
  designed.
- **Files:** auth middleware, rate limiter and Core server.
- **Tests:** query-token rejection and existing auth/CSRF tests.

### Command execution

- **Problem:** every non-flag operand was accepted, destructive generic binaries
  were present, output was unbounded, and timeout/stream completion behavior was
  inconsistent.
- **Risk:** privilege abuse, option/operand injection, memory exhaustion,
  orphaned/hung processes and lost output.
- **Fix:** executable and operand policies, strict service/package/log
  validation, removal of generic rm/chown/chmod/file mutation, fixed
  environment, caller/default deadlines, long-operation bounds, 1 MiB output
  limits and synchronized stream shutdown.
- **Files:** `internal/agent/executor/shell.go`.
- **Tests:** rejected shell/path/service/package/log/Git inputs, output bound and
  deadline preservation.

### Agent recovery and error disclosure

- **Problem:** gRPC panic interceptors existed but were not installed and panic
  values/internal operation errors were returned to clients.
- **Risk:** Agent crash and disclosure of privileged paths/output.
- **Fix:** unary/stream recovery interceptors installed, client-safe errors, 8
  MiB gRPC message limit and structured server-side panic logging.
- **Files:** Agent application/server and platform recovery middleware.
- **Tests:** panic recovery does not leak the panic value.

### Filesystem containment and permissions

- **Problem:** allowed roots included all of `/etc` and `/home`; validation was
  lexical; root deletion, world-writable modes and UID/GID 0 were possible.
- **Risk:** arbitrary root file overwrite, authorized_keys/systemd/sudoers
  modification, symlink escape and unsafe ownership/modes.
- **Fix:** least-privilege managed roots, absolute/clean paths, existing symlink
  ancestry resolution, allowed-root deletion refusal, atomic writes, no
  world-write/special bits, no root ownership and cross-platform ownership
  metadata implementation.
- **Files:** Agent filesystem manager and OS-specific ownership files.
- **Tests:** root boundary, traversal, symlink escape, root deletion, atomic
  replace and unsafe chmod.

Residual risk: path resolution and mutation are separate syscalls. Linux
`openat2`/directory-FD operations are still recommended for hostile concurrent
writers; current managed configuration parents are root-controlled, reducing
but not mathematically eliminating TOCTOU.

### Archive security

- **Problem:** extraction delegated directly to tar/unzip/7z.
- **Risk:** Zip Slip, tar traversal, link/device creation, system overwrite and
  archive bombs.
- **Fix:** zip/tar/tar.gz extraction is entry-by-entry in Go with destination
  containment, link/special-file rejection, 10,000-entry and 1 GiB limits,
  bounded permissions and context cancellation. Unsupported safely undecodable
  formats are rejected.
- **Files:** Agent archive manager and server-side filesystem resolution.
- **Tests:** traversal, Zip Slip, tar symlink rejection and regular extraction.

### Configuration transactions

- **Problem:** predictable `.tmp`/`.bak` names, symlink targets, incomplete
  rollback and unbounded validator output.
- **Risk:** temporary-file attacks and configuration/service left broken after
  reload failure.
- **Fix:** same-directory unique files, fsync, symlink rejection,
  prepare/validate/commit/reload/restore/reload workflow, safe no-original
  rollback, bounded validator execution and diagnostics.
- **Files:** `internal/platform/config/manager.go`.
- **Tests:** successful commit, reload rollback and symlink rejection.

Nginx site application and Fail2ban presets retain their existing compensating
rollback logic. A single typed Agent transaction RPC should ultimately replace
module-local orchestration for PHP/Fail2ban/firewall to close the remaining
cross-process failure window.

### Tasks and restart behavior

- **Problem:** persisted module jobs could remain `pending`/`running` forever;
  work used an unbounded detached context.
- **Risk:** lost/stuck operations and false state after restart.
- **Fix:** 30-minute operation deadline; startup recovery marks interrupted jobs
  terminally failed with an explicit reconciliation message. Automatic replay
  is intentionally forbidden until each operation has an idempotency contract.
- **Files:** module service and Fx startup.
- **Tests:** pending and running recovery.

Cancellation/retry/progress are not exposed in this stage because that would be
new user functionality. The durable state and safe restart semantics are now a
foundation for that later API.

### OS compatibility and identity

- **Problem:** unknown hosts silently became Ubuntu and site ownership fell back
  to UID/GID 33.
- **Risk:** ownership assigned to the wrong account.
- **Fix:** exact `/etc/os-release` parsing for Ubuntu, Debian, RHEL, CentOS,
  Rocky, AlmaLinux and Fedora families; unknown OS is an error; numeric fallback
  removed. Installer selects apt or dnf/yum and rejects unsupported systems.
- **Files:** OS provider, site service and installer.
- **Tests:** supported family matrix and unknown-OS rejection.

### Installer and updater

- **Problem:** updater used `sh -c` with a pipeline; release extraction trusted
  archive paths.
- **Risk:** shell execution complexity, partial downloads and archive traversal.
- **Fix:** bounded HTTPS download to a mode-0700 temporary file, fixed
  `/bin/sh <file>` invocation with clean environment, exact update-request
  parsing, cleanup after success, exact three-file release manifest and
  ownership/permission-neutral extraction.
- **Files:** CLI and `install.sh`.

Checksums are verified as before. Cryptographic signing independent of the
GitHub release account remains a v1.0 release-chain requirement.

### Health checks and privileged module operations

- MySQL's unconditional healthy placeholder was replaced by real package and
  service checks.
- PostgreSQL, Node.js, Git and firewall checks now distinguish lookup errors,
  missing packages and stopped services.
- Certbot no longer writes temporary systemd units; it uses bounded Agent
  execution.
- Nginx site/log discovery uses typed directory/file operations instead of
  `sh`, `find`, `grep`, `test` and `truncate`.
- MySQL requires configured administrator credentials and parameterizes the
  password instead of interpolating it into SQL.

## Verification

- Go tests: all packages pass except two SQLite migration runtime tests on the
  Windows audit host because `go-sqlite3` requires CGO; the migration package
  must be rerun in Linux CI.
- Go vet: passes for the same package set.
- Frontend: 3 tests pass, ESLint passes and production build passes.
- Security regression tests added across executor, filesystem, archive, RBAC,
  recovery, configuration transactions, tasks and OS providers.
- Shell syntax could not be locally checked because Bash is unavailable on the
  Windows audit host; Linux CI is required.
- `npm audit` was not executed because the managed environment rejected sending
  dependency metadata to the public npm registry without separate explicit
  authorization.

## Remaining release gates

1. Linux race/CGO suite and clean-host matrix on every claimed distribution.
2. Directory-FD/`openat2` filesystem operations for strict TOCTOU resistance.
3. Typed Agent transaction RPC covering PHP, Fail2ban and firewall
   prepare/validate/commit/rollback.
4. Operation-specific idempotency, cancellation, retry and progress contracts.
5. Independent release signing and verified rollback.
6. Enforced dependency-audit policy in CI.

The platform foundation is materially safer, but completion of these gates is
required before a production-ready claim.
