# Contributing

OpenDeploy welcomes focused issues and pull requests.

## Development

Requirements: Go 1.25+, Node.js 22+, npm, and CGO with a C compiler for SQLite.
On Ubuntu and Debian, `make build` installs missing system packages through APT.

```bash
go test ./...
go vet ./...
cd web
npm ci
npm run build
```

Linux is the supported runtime and authoritative backend test environment. The
current source has a known Windows compile issue in filesystem ownership
metadata; use WSL or Linux CI for backend verification until it is closed.

New HTTP routes require authentication and authorization tests. New Agent
operations must validate resource operands, reject symlink/path escapes and
cover failure/rollback behavior. Prefer typed RPCs over generic
`CommandExecute`.

Keep privileged operations inside the Agent and expose typed contracts rather
than arbitrary command execution. Add tests for validation, failure, and
rollback paths. Update `CHANGELOG.md` and relevant documentation with behavior
changes.

Use conventional, imperative commit subjects. By contributing, you agree that
your contribution is licensed under the MIT License.
