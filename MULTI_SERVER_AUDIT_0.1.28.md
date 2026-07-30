# OpenDeploy Multi-Server technical audit

Audit date: 2026-07-30  
Audited revision: `v0.1.27` (`aa75970`)  
Target release: `v0.1.28`

## Executive summary

The multi-server migration is incomplete. The transport foundation is present:
the Agent opens an authenticated bidirectional gRPC stream, Core tracks the live
session, the browser sends `X-Server-ID`, and a generic capability client can
dispatch a substantial set of typed `AgentClient` operations through the stream.

The primary defect is wiring. Core constructs the routing client in
`internal/core/app/fx.go`, but Sites, Services, the module loader and updater are
explicitly injected with the concrete local `*agentclient.Client`. These features
therefore execute on the Core host even when a remote server is selected. A
fail-closed middleware exists, but is not mounted. The frontend capability warning
is cosmetic: it neither prevents a page from mounting nor stops its requests.

The remote Agent version is also lost on the stream path. Agent sends it in
`AgentHello`, but `controlplane.session` retains only server ID and capability
names. Stream heartbeat persistence has no version, so the UI displays `—`.

Overall multi-server readiness is approximately **42%**. Control-plane transport
is approximately **80%** ready; page-level remote execution is approximately
**30%** ready.

## Remediation progress

The findings above describe the audited `v0.1.27` baseline. The following items
have since been completed on `main`:

- fail-closed backend routing and frontend view isolation for unsupported remote
  features;
- capability-aware dependency injection for Services and runtime modules;
- an authoritative, versioned registry for implemented Agent capabilities, with
  pre-dispatch enforcement;
- remote enablement of Dashboard, Processes, Services, Firewall and Cron;
- Agent/API version preservation from authenticated handshake and heartbeat;
- typed offline, timeout and capability errors;
- server-context propagation into detached module work;
- ControlPlane reconnect cancellation, backpressure and duplicate-result
  hardening.
- typed remote Nginx site apply with Agent-owned validation, atomic rollback and
  reload; generic `command.execute` remains unavailable remotely;
- typed remote site-root ownership, PHP socket inspection, HTTP health probing
  and Certbot issuance primitives.
- automatically enabled gRPC Control Plane on port 5889 with a persistent,
  Go-native ECDSA CA and server identity (no manual `openssl` bootstrap);
- CA-signed Agent enrollment identities, certificate pinning against the
  enrollment database, and a compatibility path for previously issued
  self-signed Agent certificates;
- legacy Agent update migration via the authenticated
  `/api/v1/agents/control-plane` profile endpoint;
- explicit separation of legacy HTTP heartbeat health from live Control Plane
  readiness, including connection ID, Agent/API versions, timestamps and
  actionable diagnostics in the server details UI;
- bounded hello and heartbeat timeouts, duplicate-session replacement,
  disconnect-on-delete and Agent clock-skew validation.

Still intentionally blocked until dedicated contracts are designed: Sites and
general Modules orchestration, Settings, Packages UI, Terminal, Network, and
unification of module jobs with remote tasks/events/logs. Site HTTP routes remain
fail-closed while a remote pre-change backup contract is implemented; the
runtime provisioning commands above are therefore not advertised as complete
`sites` page support yet.

## Exact request path and root cause

1. `web/src/api/client.js:42-46` adds `X-Server-ID`.
2. `internal/core/server/server.go:183-188` applies
   `servercontext.Middleware`, which stores it in the request context.
3. `internal/core/capability/client.go:24-33` contains the intended local/remote
   dispatcher and calls `controlplane.Manager.Dispatch`.
4. `internal/core/app/fx.go:57-61` registers it as `contract.AgentClient`.
5. `internal/core/app/fx.go:251-265` bypasses it for Sites, Services and updater;
   `internal/core/app/fx.go:350-367` bypasses it for all runtime modules.
6. `internal/core/servercontext/context.go:29-44` implements a fail-closed guard,
   but `RequireMigratedCapability` has no production call site.
7. `web/src/layouts/AppLayout.vue:164-173` only renders a warning.

Thus “This feature is not supported by the selected server” originates in the
frontend, based on `/servers/{id}/capabilities`. Local information appears because
several backend services still hold the local Unix-socket Agent client and no
active backend guard rejects the request.

## Page audit

| Panel area | Current remote behavior | Agent/control-plane support | Status / defect |
|---|---|---|---|
| Dashboard | Overview and process calls use routed `contract.AgentClient`; snapshots are server-scoped DB rows | `system.stats`, process list/kill | Partial; WebSocket is outside standard auth/context middleware |
| Processes | Routed through Dashboard service | Complete | Works |
| Sites | DB rows are server-scoped; host operations use concrete local Agent | File primitives exist; no site orchestration command | **P0: local Core execution** |
| Services | DB rows are server-scoped; systemd operations use concrete local Agent | Status/actions/logs exist | **P0: local Core execution** |
| Modules | State/jobs are server-scoped; global module singletons receive local Agent | Only lower-level primitives exist | **P0: local Core execution** |
| Firewall | Module receives local Agent | Commands exist | **P0: advertised remote capability is bypassed** |
| Cron | Module receives local Agent; templates/import/export are module-local | Commands exist | **P0: advertised remote capability is bypassed** |
| Files | Exposed through Site endpoints, whose service receives local Agent | Full basic file/archive primitives | **P0: remote capability exists but is bypassed** |
| Certificates | No dedicated page or REST surface | No stream command | Not implemented; falsely advertised |
| Settings | Values are server-scoped DB rows; update/backup actions are Core-local | No settings command | Partial/stub; remote settings are not machine settings |
| Users | Core-wide control-plane users | Not applicable | Correctly Core-global |
| Tasks | Module jobs are server-scoped | Task-progress message exists | Partial; module jobs and remote tasks are separate models |
| Logs | Core DB logs are server-scoped | File/service log primitives | Partial; page is not a remote Agent log view |
| System | No dedicated page; Dashboard supplies stats | `system.stats` | Partial |
| Network | No route, endpoint or command | None | Not implemented |
| Packages | No page/REST handler | status/install/remove/update | Agent-only, unused |
| Terminal | No page/REST handler | Generic command execution only | Not implemented as a terminal |
| Update | Settings page controls Core updater | No remote-update capability | Local Core only; unsafe under remote context |

The actual Vue router contains Dashboard, Servers, Modules, Sites, Services,
Settings, Tasks, Cron, Processes, Logs, Users and Firewall. Files are embedded in
Sites. Certificates, System, Network, Packages, Terminal and a dedicated Update
page do not exist.

## Capability registry audit

There is no authoritative registry. Three independent lists exist:

- Agent advertisement: `internal/agent/remote/stream.go:112-117`;
- local-server API response: `internal/core/remote/handler.go:37-51`;
- frontend route mapping: `web/src/layouts/AppLayout.vue:164-168`.

They disagree. Agent advertises `dashboard`, `sites`, `modules`, `processes`,
`services`, `files`, `firewall`, `cron`, `certificates`, `logs`, `packages`,
`tasks`, `settings`, `events`, and `system`. The local API reports only
`dashboard`, `processes`, `services`, `files`, `firewall`, `cron`, `packages`,
and `system`.

The stream command router actually implements:

- system stats and process list/kill;
- service status/actions/logs and log subscription;
- file read/write/delete/rename/copy/chmod/chown/mkdir/list/archive/logs;
- firewall status/list/rule/delete/toggle/reset;
- Cron list/get/create/update/delete/enable/disable/run/history/validate;
- package status/install/remove/update;
- generic command execution and ping.

It does not implement site, module, certificate, task, settings or event commands.
Task progress and events are Agent-to-Core messages, not dispatchable
capabilities. `dashboard`, `logs` and `system` are aliases for lower-level command
kinds rather than versioned registry entries.

`Manager.Dispatch` does not call `HasCapability` before enqueueing a command
(`internal/core/controlplane/manager.go:199-239`). Rejection happens only at the
Agent's default `unsupported control-plane command` branch.

| Capability | Advertised | Agent implementation | Used remotely by current HTTP path |
|---|---:|---:|---:|
| dashboard/system stats | Yes | Yes | Yes |
| processes | Yes | Yes | Yes |
| services | Yes | Yes | No: DI bypass |
| files | Yes | Yes | No: DI bypass |
| firewall | Yes | Yes | No: module DI bypass |
| cron | Yes | Yes | No: module DI bypass |
| packages | Yes | Yes | No HTTP/UI consumer |
| sites | Yes | No site command | No |
| modules | Yes | No | No |
| certificates | Yes | No | No |
| tasks/settings/events | Yes | Message-only or absent | No |
| network/terminal | No | Absent/generic command only | No |

## REST routing audit

All protected requests receive server context, but only these execution paths are
truly migrated:

- `GET /api/v1/dashboard`;
- `GET /api/v1/system/processes`;
- `POST /api/v1/system/processes/{pid}/kill`.

`/sites/**`, `/services/**`, `/modules/**`, `/jobs/**`, `/tasks/**`,
`/settings` and `/logs` are at most context-aware at their SQLite repository
boundary; their host operations are not completely migrated.

`/auth/**`, `/users/**` and `/servers/**` are Core-global by design and should
explicitly opt out of target-server semantics. Core update and backup endpoints
must also be explicitly local-only.

No REST endpoints exist for standalone files, certificates, packages, terminal,
network or remote system management. Cron and Firewall module routes exist, but
their instances use the local Agent because of module bootstrap wiring.

The dashboard WebSocket route is registered at
`internal/core/server/server.go:169-171`, before auth and server-context
middleware. Its ticket carries a server ID, but this separate isolation mechanism
requires an explicit cross-server test.

## Handshake and Agent version audit

Implemented sequence:

1. Agent establishes TLS and presents its issued certificate.
2. First stream message is `AgentHello` with server ID, certificate fingerprint,
   protocol/API/Agent versions and capability names.
3. Core verifies TLS fingerprint and enrollment identity.
4. Core validates protocol version, registers a session and sends `StreamWelcome`.
5. Agent starts heartbeats and receives commands/subscriptions.

Only server ID and capability names survive in the live session
(`internal/core/controlplane/manager.go:24-61`). API version and Agent version are
dropped. OS and architecture come from HTTP registration, not stream hello.
There is no explicit `Ready` message/state.

`ControlPlaneHeartbeat` (`internal/core/remote/service.go:43-50`) constructs a
heartbeat without `AgentVersion`; the stream heartbeat message has no version
field. The legacy REST heartbeat payload also omits it
(`internal/agent/remote/client.go:77-86`). This is the exact reason connected
remote Agents show no version.

## Findings and roadmap

### P0 — local-host execution under remote context

- **Location:** `internal/core/app/fx.go:251-265,350-367`; missing production use
  of `servercontext.RequireMigratedCapability`.
- **Cause:** concrete local-client injection plus dead fail-closed guard.
- **Impact:** remote selection can read or mutate Core-host services, files,
  firewall, Cron, modules, sites or updater state.
- **Fix:** inject `contract.AgentClient` into Sites, Services and modules; keep
  updater/backup explicitly local-only; mount a route-aware fail-closed guard;
  test with a local client that fails if remotely called.
- **Complexity:** medium, 3–5 engineering days plus integration tests.

### P0 — capabilities are labels, not enforceable contracts

- **Location:** `stream.go:112-117`, `manager.go:199-239`,
  `remote/handler.go:37-51`.
- **Cause:** hand-maintained top-level names are unrelated to command kinds.
- **Impact:** false frontend availability and late runtime errors.
- **Fix:** one versioned registry mapping page capabilities to exact command
  kinds; derive hello/API responses; validate before dispatch.
- **Complexity:** medium.

### P0 — frontend warning does not isolate data

- **Location:** `AppLayout.vue:85-87,164-173`.
- **Cause:** informational banner leaves the view mounted.
- **Impact:** unsupported pages continue sending requests.
- **Fix:** prevent route/view mounting; retain backend authority.
- **Complexity:** small.

### P1 — remote Agent version is dropped

- **Location:** `manager.go:24-61`, `remote/service.go:43-50`,
  `agent/remote/client.go:77-86`.
- **Fix:** retain hello metadata, persist it after authentication, include version
  in both heartbeat transports, and expose one canonical server projection.
- **Complexity:** small.

### P1 — incomplete page migration

Enable and test Services, Files, Firewall and Cron first. Sites and Modules need
explicit orchestration contracts. Certificates, Packages, Terminal, System and
Network need API/product design before implementation.

### P2 — split task and event models

Legacy REST tasks, stream task progress and module jobs are separate. Define one
task identity/state model with transport adapters.

### P2 — lifecycle and backpressure

Bound queues can block; `resolveChunk` silently drops chunks when full. Add
backpressure semantics, typed remote errors, compatibility ranges and load/race
tests.

### P3 — eliminate duplicated capability knowledge

Generate frontend metadata and documentation from the registry. Add static tests
that every advertised command has an Agent handler and every remote route declares
its required command set.

## Recommended delivery order

1. Fail closed for every remote request except the three verified paths.
2. Correct dependency injection for Sites, Services and modules.
3. Add a versioned command registry and remove false advertisements.
4. Persist hello metadata and repair Agent version display.
5. Enable Services, Files, Firewall and Cron one at a time with isolation tests.
6. Design site/module orchestration commands rather than using arbitrary shell as
   their long-term API.
7. Unify tasks/events/logs, then implement remaining product pages.

## Acceptance criteria for implementation releases

- Every remote protected endpoint dispatches to that exact Agent or returns typed
  `501 capability_unavailable`; it never calls the local Unix-socket Agent.
- Offline Agents fail without local fallback.
- Every advertised capability has a version and an Agent contract test.
- Server list/details show version from stream and legacy heartbeat paths.
- WebSocket tickets cannot cross server rooms.
- Switching server cancels and reloads queries, polling and subscriptions under
  the new server ID.
