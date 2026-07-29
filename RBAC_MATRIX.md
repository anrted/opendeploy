# OpenDeploy RBAC matrix

This matrix reflects the server-side middleware enforced by the REST API.
Frontend visibility is a usability aid only; authorization is always repeated
by Core before any Agent RPC is invoked.

| Area / operation | Viewer | Operator | Administrator |
|---|---:|---:|---:|
| Dashboard and system metrics | Read | Read | Read |
| Processes | Read | Read, terminate non-protected | Read, terminate non-protected |
| Modules, Firewall, Nginx, Fail2Ban, Cron | Read | Read, configure, enable/disable | Full lifecycle |
| Sites and site files | Read | Create, update, delete | Create, update, delete |
| Services and logs | Read | Read, manage | Read, manage |
| Tasks | Read | Read, cancel, retry, delete | Read, cancel, retry, delete |
| Settings | Read | Read | Read, update, security operations |
| Updates and backup restore | Read status/history | Read status/history | Apply, rollback, create/restore |
| Users and sessions | None | None | Manage |
| Audit log | Read | Read | Read |

## Agent boundary

The Agent accepts requests only from Core over its protected gRPC transport. It
does not receive browser JWTs and therefore cannot replace Core RBAC checks.
Typed Agent subsystems still enforce operation-level safety: filesystem roots,
command allowlists, firewall validation, Cron validation, archive boundaries,
and protected-process rules.

## Protected processes

PID 1, the Agent itself, OpenDeploy Core/updater, named critical Linux
processes, kernel workers, and processes in a systemd `.service` cgroup cannot
be terminated through Process Manager. System services must be controlled
through the Services API.
