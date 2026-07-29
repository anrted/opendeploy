# Changelog

All notable changes to OpenDeploy are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## Unreleased

## [0.1.25] - 2026-07-30

### Added

- Persistent Agent-initiated bidirectional gRPC control plane with TLS client
  certificates, reconnect backoff, command correlation, heartbeat, events,
  task progress, subscriptions, and streamed log chunks.
- Unified server context and capability client for local and remote execution.
- Server switcher with persisted selection and automatic API context.
- Capability inventory endpoint and unsupported-feature indication.
- Server-scoped Sites, domains, Services, Modules, Tasks, Settings, audit,
  logs, and Dashboard history.
- Remote capability adapters for system metrics, processes, systemd, files,
  archives, packages, Firewall, Cron, and live service/file logs.
- Automatic control-plane configuration during remote Agent enrollment.

### Changed

- Localhost is represented as an automatically registered server and uses the
  same capability contract as remote Agents.
- Dashboard WebSocket tickets are bound to the selected ServerID.
- Long-running module jobs preserve Server Context after the HTTP request ends.

### Security

- Remote control-plane sessions require possession of the registered Agent
  private key and verify the authenticated TLS peer certificate fingerprint.
- Remote filesystem, package, service, process, Firewall, and Cron operations
  continue to use typed Agent validation and Core RBAC.

## [0.1.23] - 2026-07-30

### Added

- Infrastructure / Servers control plane for managing remote OpenDeploy Agents.
- One-time, expiring enrollment tokens and agent-generated certificate signing
  requests with certificate-bound heartbeat authentication.
- Remote server inventory, heartbeat metrics, health state, event history,
  task queue, maintenance mode and lifecycle actions.
- Responsive server list, enrollment wizard and server detail dashboard.
- Hardened `install-agent.sh` for Ubuntu/Debian amd64 and arm64 hosts.

### Fixed

- PHP sites now prioritize `index.php` over a leftover `index.html`.
- Release publishing now uses the Node.js 24-compatible release action.

## [0.1.22] - 2026-07-30

### Fixed

- Registered the system logs API at `/api/v1/logs`.
- Allowed the Agent to perform strictly bounded PHP-FPM socket readiness checks.
- Reconciled active site PHP pools and Nginx configurations during startup so
  sites created by older versions are repaired automatically.
- Correctly transition sites from PHP to static or proxy mode, remove obsolete
  PHP-FPM pools, and clear stale PHP version metadata.
- Updated GitHub Actions to Node.js 24-compatible action releases.

## [0.1.19] - 2026-07-29

### Added

- Firewall rule editing with conflict detection and IPv4, IPv6, CIDR, port
  range, protocol, direction, action, and comment validation.
- Explicit Firewall reload operation and post-toggle state verification.
- Protected-process policy for PID 1, OpenDeploy, critical Linux, kernel, and
  systemd-managed processes.
- Server and frontend RBAC matrices, including a dedicated process-management
  permission.
- Uniform API error envelopes with codes, details, recommendations, correlation
  IDs, and global frontend toast notifications.

### Changed

- `core.default_php` now supplies the PHP version only when creating a PHP site
  that does not specify one explicitly.
- Firewall changes are validated by Core and revalidated at the privileged
  Agent boundary.

### Fixed

- Prevented Viewer accounts from terminating processes.
- Prevented Process Manager from terminating OpenDeploy or critical host
  services and replaced opaque failures with an actionable conflict response.
- Fixed Firewall disable/reload status drift and surfaced real UFW failures.
- Removed the unsupported development-channel update action from Settings.

## [0.1.18] - 2026-07-29

### Added

- Cron module with typed Agent RPCs, atomic managed crontab updates, privilege
  dropping and bounded execution history.
- Cron CRUD, validation, enable/disable, manual run, duplication, templates,
  JSON/YAML/crontab import and export.
- Read-only discovery of system and per-user crontabs.
- Task Manager integration for manual Cron runs.
- Dedicated Cron UI with search, filters, sorting, pagination, schedule
  builder and automatically refreshed logs.
- Backup and recovery coverage for managed Cron configuration and state.

### Added

- Live Fail2Ban protection preset cards with jail, log, threshold, ban duration,
  rule count, modification time, details and localized configuration dialogs.
- Typed protection-preset APIs for listing, previewing, saving, resetting and
  toggling managed presets.
- Transactional preset validation and rollback tests.
- Real Nginx overview diagnostics, health checks, configuration explorer,
  certificate metadata/renewal, and managed settings.
- Transactional rollback tests for Nginx sites, settings and configuration
  edits.

### Fixed

- Preserved customized Fail2Ban preset settings while disabled and validated
  active changes before restarting the service.
- Expanded the Nginx scan and PHP exploit filters to cover documented sensitive
  paths, traversal attempts, shells, installers, dumps and backup archives.
- Implemented the current data-grid provider contract for Nginx virtual hosts
  and certificates.
- Replaced hard-coded Nginx settings and silently rejected diagnostic commands
  with validated Agent-backed operations.

## [0.1.16] - 2026-07-29

### Added

- Arbitrary IPv4/IPv6 entry to the banned-IP grid, backed by a persistent
  all-ports Fail2Ban jail with an unlimited ban time.
- An Nginx bad-bot preset for clients that explicitly identify as common
  offensive scanners, including `foda-scanner`.
- Generic input metadata for data-grid actions.

### Changed

- Expanded the managed Fail2Ban PHP probe filter to catch repeated 404 scans for
  arbitrary PHP web-shell names and WordPress paths.
- Added an update action for servers where the PHP probe preset is already
  enabled but its managed configuration is outdated.

## [0.1.14] - 2026-07-28

### Added

- Complete Russian localization for the user, firewall, module, log, task,
  service, site, settings, dashboard, and File Manager interfaces.
- Runtime availability metadata for module actions.

### Fixed

- Limited uninstalled module pages to the Overview section until installation.
- Reflected the actual enabled state of Fail2Ban protection presets and disabled
  actions that do not apply to the current state.
- Replaced placeholder Fail2Ban jail and banned-IP rows with live
  `fail2ban-client` data.
- Added working per-IP unban actions and global unban-all handling.
- Reloaded the correct data-grid schema and rows when switching between the
  Fail2Ban Jails and Banned IP sections.
- Rolled back preset configuration if enabling Fail2Ban at boot fails.

## [0.1.13] - 2026-07-27

### Fixed

- Restored all log-viewer and module-action icons with bundled SVG components.
- Kept the mobile sidebar close control hidden until the sidebar is open and
  constrained the language selector within narrow headers.
- Read the Fail2Ban machine-readable version instead of rendering its CLI help.
- Populated runtime health for every module from its health report instead of
  displaying `unknown`.

## [0.1.12] - 2026-07-27

### Added

- Runtime reconciliation for software installed outside OpenDeploy.
- Administrator-triggered GitHub updates through a dedicated systemd path unit.
- Full repository readiness/security audit and release-gated roadmap.
- Stage 1 security foundation: granular module RBAC, bounded process execution,
  restricted filesystem roots, safe archive extraction, configuration
  transactions, job restart reconciliation and OS-family detection.

### Security

- Removed JWT query-string authentication and generic destructive Agent commands.
- Prevented archive traversal/link extraction and broad `/etc`/`/home` writes.
- Installed gRPC panic recovery and stopped leaking privileged Agent errors.
- Removed hardcoded UID/GID fallback, temporary Certbot systemd units and
  interpolated MySQL passwords.

### Documentation

- Rewrote architecture documentation around the implemented request, task,
  site, installation and update lifecycles.
- Corrected overstated production, setup-wizard, module and portability claims.
- Documented the refactored service boundaries, domain event flow and remaining
  architecture follow-up items.

### Changed

- Split the site, module-manager and Nginx implementations into cohesive
  services while preserving their public API and behavior.
- Reduced the File Manager screen to a coordinator backed by reusable Vue
  components and composables.
- Added typed domain events and isolated subscribers for site lifecycle,
  dashboard updates and audit integration.

## [0.1.0-alpha] - 2026-07-25

### Added

- Core, privileged System Agent, CLI, embedded Vue 3 interface, and SQLite storage.
- Authentication with short-lived JWT access tokens and rotating refresh tokens.
- Permission-based RBAC for dashboard, modules, sites, services, and settings.
- One-time, short-lived authentication tickets for dashboard WebSocket connections.
- Nginx, PHP, Node.js, and Git modules.
- Dashboard metrics, module management, systemd service management, site management,
  settings, audit logging, and consistent API errors.
- Transactional Nginx site changes with validation and rollback.
- Production systemd units and an idempotent Ubuntu installer/uninstaller.
- Ubuntu end-to-end smoke workflow covering installation, login, site provisioning,
  Nginx validation, and service restart.
### Security

- Core never executes privileged system commands; all privileged operations cross
  the local Agent gRPC boundary.
- The Agent Unix socket is restricted to `root:opendeploy` with mode `0660`.
- Systemd hardening and dedicated unprivileged Core service account.
- Validated package, systemd, filesystem, domain, and Nginx operations.
- Argon2id password hashing, rate limiting, audit records, and confirmation flows
  for destructive UI actions.
- WebSocket bearer tokens are no longer exposed in URLs.
- Updated chi, gRPC, and Go cryptography/network dependencies to patched
  releases after reviewing GitHub security advisories.

[0.1.0-alpha]: https://github.com/anrted/opendeploy/releases/tag/v0.1.0-alpha
[0.1.12]: https://github.com/anrted/opendeploy/releases/tag/v0.1.12
[0.1.13]: https://github.com/anrted/opendeploy/releases/tag/v0.1.13
[0.1.14]: https://github.com/anrted/opendeploy/releases/tag/v0.1.14
[0.1.25]: https://github.com/anrted/opendeploy/releases/tag/v0.1.25
