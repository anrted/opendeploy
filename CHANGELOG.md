# Changelog

All notable changes to OpenDeploy are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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

[0.1.0-alpha]: https://github.com/anrted/opendeploy/releases/tag/v0.1.0-alpha
