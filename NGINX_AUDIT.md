# Nginx Module Audit

Audit date: 2026-07-29

## Current capability matrix

| Area | Status | Notes |
| --- | --- | --- |
| Package install/remove | Complete | Runs through asynchronous module jobs and the typed Agent package API. |
| Service enable/disable/start/stop/restart | Complete | Uses the typed systemd Agent API. Runtime action availability follows the active service state. |
| Configuration test/reload | Complete | Reload is blocked unless `nginx -t` succeeds. |
| Overview | Complete for available host data | Reports package/service state, version, systemd substate, PID, Nginx process CPU/memory, process count, start time, active connections when `stub_status` is configured, virtual-host/certificate counts, worker settings, config path and build string. |
| Health check | Complete | Checks package/version, systemd, `nginx -t`, readable main config, Nginx process, listening HTTP(S) ports and configured certificate validity. |
| Logs | Complete with polling | Reads systemd, global access/error logs and discovered custom `*.log` files. UI supports selection, line limit, polling, search, level filtering, download and safe file clearing. A persistent streaming transport is not implemented. |
| Settings | Complete for supported directives | Reads effective configuration and manages worker processes/connections, keepalive, body size, sendfile, gzip/types, server tokens and global access/error logs. Writes are validated and rolled back after failed validation or reload. |
| Virtual hosts | Operational | The module lists managed configs and supports enable, disable and delete with rollback. Creation and business-level editing remain canonical operations of the shared Sites API/UI so database and filesystem state cannot diverge. |
| Certificates | Operational metadata and renewal | Lists certificates referenced by managed Nginx sites, reads public X.509 issuer/dates/SAN/status/path, and renews Let's Encrypt certificates. Import, self-signed issuance and destructive deletion are not exposed by this module. |
| Configuration explorer | Complete for managed paths | Lists and searches `nginx.conf`, `mime.types`, `conf.d`, snippets and managed site files. Supports full-content editing, validation, reload and rollback. Enabled-site copies are read-only to preserve the sites-available source of truth. |
| HTTP/2 | Site-template support | Existing TLS site templates use the platform Nginx syntax. No global boolean is exposed because HTTP/2 is a per-listener setting. |
| HTTP/3 | Not exposed | Requires an Nginx build with QUIC support, UDP firewall changes and per-site certificate/listener configuration. It cannot be represented safely as a global toggle. |
| Stream context | Configuration editor only | Visible/editable through `nginx.conf`; no separate stream-object model exists. |

## Removed placeholders and defects

- Replaced the obsolete data-grid methods that prevented Virtual Hosts and
  Certificates APIs from working.
- Removed the empty certificate response and non-functional delete action.
- Replaced hard-coded setting values and no-op saving with real configuration
  reads and transactional writes.
- Replaced rejected `ps` diagnostics with the typed process API.
- Restricted socket, local status, build and X.509 diagnostics to explicit
  read-only Agent command forms.
- Added rollback for site upsert, enable, disable, delete, settings and manual
  configuration edits.

## Remaining production work

- Add a typed log-stream RPC if true push-based `tail -f` is required instead
  of the current three-second polling.
- Build a shared certificate domain service before adding import, self-signed
  issuance, deletion and operation history. Implementing those directly in the
  Nginx module would duplicate Certbot/Sites state and risk deleting
  certificates shared by multiple virtual hosts.
- Add an explicit QUIC capability API before offering HTTP/3 controls.

Readiness after this work is estimated at 85%. Core lifecycle, diagnostics,
logs, settings, managed configuration, virtual-host operations and certificate
inspection are functional. The remaining work is isolated to streaming,
advanced certificate lifecycle and HTTP/3 capability orchestration.
