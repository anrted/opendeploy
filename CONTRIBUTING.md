# Contributing

OpenDeploy welcomes focused issues and pull requests.

## Development

Requirements: Go 1.23+, Node.js 22+, npm, and CGO with a C compiler for SQLite.

```bash
go test ./...
go vet ./...
cd web
npm ci
npm run build
```

Keep privileged operations inside the Agent and expose typed contracts rather
than arbitrary command execution. Add tests for validation, failure, and
rollback paths. Update `CHANGELOG.md` and relevant documentation with behavior
changes.

Use conventional, imperative commit subjects. By contributing, you agree that
your contribution is licensed under the MIT License.
