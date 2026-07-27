# Changelog

All notable changes to OpenDeploy are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## Unreleased

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
