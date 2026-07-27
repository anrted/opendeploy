# Security Policy

## Supported versions

OpenDeploy is currently pre-release software. Security fixes are applied to
the latest revision of the `main` branch.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Use GitHub's
**Security → Report a vulnerability** flow for
`https://github.com/anrted/opendeploy`.

Include affected versions, reproduction steps, impact, and any suggested
mitigation. Please allow maintainers reasonable time to investigate before
public disclosure.

## Security model

- Core runs as an unprivileged user.
- Privileged operations cross a local Unix-socket gRPC boundary.
- Agent operations are typed and command execution uses an explicit allowlist.
- Filesystem paths are lexically restricted and configuration writes are atomic.
- Nginx configuration is validated and rolled back before a failed change can
  remain active.
- Initial startup requires an administrator password supplied through
  `OD_ADMIN_PASSWORD`; no default password is provided.

Production deployments must set a random `OD_JWT_SECRET` of at least 32 bytes,
terminate TLS at a trusted reverse proxy, and restrict access to port 5888.

## Pre-release limitations

The 2026-07-27 audit found authorization, command, filesystem and archive
issues. Stage 1 closed the direct RBAC/operand/archive/broad-root problems and
added regression tests; see [PRODUCTION_STAGE1.md](PRODUCTION_STAGE1.md).
Strict directory-FD TOCTOU resistance, cross-module Agent transactions and
signed releases remain production gates. Until they are complete, use
OpenDeploy only on controlled hosts in a trusted network.

Release updates do not yet have a complete signed-artifact and automatic
rollback verification chain.
