# Multi-server and control-plane architecture

## Audit summary

Before this refactoring, OpenDeploy had two unrelated remote-control paths:

1. Core connected to the co-located Agent through unary gRPC over a Unix socket.
2. A remote Agent periodically called REST registration and heartbeat endpoints.
   The heartbeat response contained a small polling-based task queue.

The remote inventory UI therefore did not represent a usable execution target.
Every operational Vue page called an unscoped local endpoint, and Core always
used the single Unix-socket Agent client.

## Current request routing

The browser now stores one selected server ID and adds `X-Server-ID` to all
operational API requests. Inventory requests opt out of this context. Switching
the server remounts the current route, so an open Dashboard, Processes, Sites,
or other page reloads without a browser refresh.

Core converts a missing header to the stable `local` target. The value is
carried in request context; handlers never need to inspect HTTP headers.
`local` is included automatically in the Servers inventory and cannot be
deleted.

## Bidirectional control plane

`ControlPlane.Connect` is the remote transport. Agent initiates one TLS gRPC
stream and sends `AgentHello` first. Core verifies the registered server ID and
certificate fingerprint, negotiates protocol version 1, and registers the
connection by ServerID.

The stream carries:

- registration/capabilities and welcome negotiation;
- heartbeat metrics;
- correlated commands and results;
- asynchronous task progress;
- events;
- subscriptions, chunks, cancellation, ping/pong, and protocol errors.

Core's connection manager supports concurrent dispatch, deadlines, reconnect
replacement, bounded outgoing queues, and deterministic failure of pending
commands when a connection closes. Agent reconnects with exponential backoff
and jitter. All writes to either side of a gRPC stream are serialized.

The legacy REST heartbeat remains a compatibility fallback when
`agent.control_plane_address` is not configured. It is not used when the stream
is enabled. The co-located Agent's Unix-socket API is also retained as the
local-server adapter.

## Capability audit

| Area | Existing Agent capability | Multi-server routing status |
|---|---|---|
| Dashboard metrics | `SystemStats` | Stream command implemented |
| Processes | list/kill | Stream commands implemented |
| systemd services | status/actions/log streams | Routed capability and server-scoped repository |
| Files | read/write/list/archive | Routed capability; live tails use stream subscriptions |
| Firewall | status/list/mutations | Routed typed capability |
| Cron | full CRUD/history/import/export | Routed typed capability |
| Sites | Core orchestration plus Agent filesystem/Nginx | Server-scoped repository and routed capabilities |
| Modules | Core registry plus Agent packages/services/files | Server-scoped state/jobs and routed capabilities |
| Logs | Core database and Agent streams | Server-scoped search and streamed service/file tails |
| Certificates | module-owned orchestration | Routed through module filesystem/package/service capabilities |
| Users/auth | Core control-plane concern | intentionally global, not server-scoped |
| Core settings/update/backup | Core orchestration plus Agent actions | Settings are server-scoped; privileged actions use routed capabilities |

## Required deployment configuration

Core enables the remote listener only when
`server.control_plane_port`, `server.tls_certificate`, and
`server.tls_private_key` are configured. Remote Agent uses
`agent.control_plane_address`, `agent.control_plane_ca_file`, and optionally
`agent.control_plane_server_name`. Production endpoints must use a certificate
whose chain is trusted by the configured CA file.

## Stage two

The stable `contract.AgentClient` is now implemented by one context-aware
capability client. Core services and modules consume that client and therefore
do not contain local/remote branches. The adapter selects either the local
Unix-socket transport or the registered control-plane stream.

Migration 011 partitions Sites and domains, managed Services, Module state,
Tasks, Settings, audit entries, system logs, and Dashboard snapshots by
ServerID. Existing records are assigned to `local`.

Service and file log tails use control-plane subscriptions and chunks, with
backpressure, context cancellation, and stream cleanup. Other typed capability
operations use correlated command messages. Long-running Core jobs retain
Server Context and Agent task progress/events are persisted by the control
plane.

The legacy heartbeat task queue remains only for Agents installed without a
control-plane endpoint and can be removed after the compatibility window.
