# OpenDeploy 0.1.19 technical audit

Audit date: 2026-07-29

Scope: Frontend, REST middleware, Core services, module routes, Agent gRPC
boundary, UFW integration, site/PHP defaults, process management, RBAC, error
handling, tests, installer/updater regression, and release workflows.

## Findings and root causes

### Firewall disable and status

The browser-to-Agent route existed and `FirewallToggle` reached UFW. The main
reliability defect was state handling: UFW status execution errors were
silently converted into an inactive state, and toggle success was returned
without confirming that UFW reached the requested state. This made a failed
disable indistinguishable from a successful one.

The Agent now returns actual status errors and verifies the postcondition after
enable/disable. Core repeats the status verification before responding. The UI
refreshes status immediately. Reload has a dedicated protected module endpoint
and is available only while UFW is active.

UFW enablement persists its state through the operating system's normal UFW
boot integration; OpenDeploy reads the real state again after server restart.

### Firewall rule editing

Only create/delete operations existed. Input validation was effectively
delegated to UFW and the generic command validator, so API clients could submit
invalid protocols, ranges, address families, or duplicate rules.

`PUT /api/v1/modules/firewall/rules/{id}` now supports editing action,
direction, protocol, IP version, port/range, source, destination, and comment.
Core checks existence, equivalent-rule conflicts, protected management ports,
IPv4/IPv6/CIDR syntax, port bounds, protocols, and safe comments. Agent performs
the replacement within one typed firewall RPC: it adds the validated
replacement first, removes the original, and removes the replacement again if
the original cannot be removed.

### Default PHP Version

`core.default_php` was persisted and displayed but never consumed. It was a
stub.

SiteService now reads it when a new PHP site omits `app_version`. Explicit
per-site versions take precedence. Static/proxy sites and existing sites are
unchanged. The setting does not install PHP, switch system PHP-FPM, or rewrite
existing pools. Installer behavior remains repository-dependent and does not
force a default.

### RBAC

The route for terminating a process required `dashboard:view`. Viewer holds
that permission, so hiding buttons could not prevent a direct REST request.

A dedicated `process:manage` permission now protects the mutation. Operator and
Administrator receive it; Viewer does not. Module custom routes already
separate GET (`module:view`) from POST/PUT/DELETE (`module:configure`), while
sites, services, users, settings, updates, backup restore, tasks, logs, Cron,
Nginx, Firewall, and Fail2Ban use their corresponding mutation permissions.
The checked matrix is documented in `RBAC_MATRIX.md`. Core remains the
authorization boundary; Agent accepts only Core's protected gRPC connection
and independently validates privileged operation safety.

### Process Manager

The Agent previously forwarded arbitrary positive PIDs directly to gopsutil.
Errors were collapsed into gRPC Internal and then HTTP 500. PID 1, the Agent,
Core, sshd, kernel workers, and systemd-managed daemons could be targeted.

Agent now blocks invalid/PID 1, itself, OpenDeploy binaries, named critical
Linux processes, kernel workers, and any process attached to a systemd
`.service` cgroup. Protected targets return gRPC FailedPrecondition; Core maps
that to HTTP 409 with guidance to use Services. Graceful termination continues
to use SIGTERM and force mode uses SIGKILL.

### Error handling and UI

Error writers had several incompatible JSON shapes, and some module handlers
returned every failure as HTTP 500. There was no support identifier or global
notification surface.

Core, middleware, panic recovery, Firewall, and Cron now use a common envelope:

```json
{
  "error": {
    "code": "CONFLICT",
    "message": "human-readable summary",
    "details": "safe public context",
    "recommendation": "actionable next step",
    "error_id": "correlation identifier"
  }
}
```

The identifier is also returned in `X-Error-ID` and included by HTTP request
logging. Internal causes remain server-side. The Axios client renders failures
through a global dismissible toast with the recommendation and error ID.

## API changes

- Added `PUT /api/v1/modules/firewall/rules/{id}`.
- Added `POST /api/v1/modules/firewall/reload`.
- `POST /api/v1/system/processes/{pid}/kill` now requires
  `process:manage` and returns 409 for protected processes.
- Error responses now include `details`, `recommendation`, and `error_id`.
- Existing APIs and request fields remain backward compatible.

## Added checks

- Firewall address, CIDR, range, protocol, action, direction, IP-family, and
  comment validation tests.
- Command allowlist tests for UFW reload and safe comments.
- Process critical-name and PID protection tests.
- Viewer/Operator/Administrator process-permission matrix test.
- Default PHP resolution and explicit-version precedence test.
- API error-envelope safety and correlation-header test.
- Existing backend, frontend, security, updater, module, and build tests remain
  part of CI.

## Remaining limitations and recommendations

- RBAC roles are currently fixed; custom roles and per-resource ownership
  policies remain future work.
- Process Manager exposes SIGTERM and SIGKILL. A typed multi-signal Agent API
  would be required before safely exposing SIGINT or arbitrary signals.
- UFW is the only firewall provider. nftables/firewalld need separate typed
  providers rather than shell compatibility layers.
- External changes made directly through UFW can race with a panel edit;
  current existence/conflict checks minimize this window but do not lock UFW
  globally.
- Several early-alpha modules still expose limited lifecycle functionality.
  Their maturity is tracked in the repository-wide `AUDIT.md`.
