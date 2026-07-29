# OpenDeploy

> **Early-alpha, open-source single-host Linux server management platform**

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Architecture](https://img.shields.io/badge/Architecture-Modular-brightgreen)](#architecture)
[![CI/CD](https://github.com/anrted/opendeploy/actions/workflows/ci.yml/badge.svg)](https://github.com/anrted/opendeploy/actions)

OpenDeploy is a modular server-management foundation with a Vue web interface,
an unprivileged Core and a privileged local Agent. It implements useful alpha
workflows, but does not yet fully manage a Linux server and is not production
ready. See the evidence-based [audit](AUDIT.md).

---

## ✨ Features

- 🔒 **Privilege separated** — Core is unprivileged; supported OS mutations go through a local root Agent
- 🧩 **Truly modular** — Core knows nothing about Nginx, PHP or Docker; modules register themselves
- ⚡ **Fast & lightweight** — Pure Go backend, Vue 3 + TailwindCSS frontend, embedded as single binary
- 📡 **Real-time** — WebSocket-powered live metrics, job output streaming
- 🔌 **Self-contained control plane** — SQLite and the web UI are embedded; managed services remain optional host dependencies
- 🛡️ **JWT + RBAC foundation** — Access tokens, refresh rotation and roles; authorization hardening remains before v1.0

---

## 🏗️ Architecture

```
Browser (Vue 3 SPA)
     │  HTTPS + WebSocket :5888
     ▼
OpenDeploy Core (Go)
  HTTP API · Auth · Module Registry · WebSocket Hub
     │  gRPC (Unix Socket)
     ▼
OpenDeploy Agent (Go, root)
  Shell · systemd · APT/DNF · Filesystem · UFW
```

Three independent processes:
| Process | Runs as | Responsibility |
|---|---|---|
| `opendeploy-core` | `opendeploy` user | HTTP API, auth, module registry, web UI |
| `opendeploy-agent` | `root` | Privileged system operations via gRPC |
| `opendeploy` (CLI) | any user | Command-line management |

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design decisions.
See [BACKUP_RECOVERY.md](BACKUP_RECOVERY.md) for backup and clean-server
disaster recovery procedures.

---

## 🚀 Quick Start

### Requirements

- Linux (Debian/Ubuntu or RHEL/CentOS/Fedora)
- Go 1.23+ (for building from source)

### Install

OpenDeploy can be evaluated on a disposable Ubuntu 22.04+ host:

```bash
curl -fsSL https://raw.githubusercontent.com/anrted/opendeploy/main/install.sh | sudo bash
```

Set the initial administrator credentials as described in
[INSTALL.md](INSTALL.md), then open `http://YOUR_SERVER_IP:5888`.

> ⚠️ OpenDeploy is early alpha software. Use it on a test server and place it
> behind HTTPS before exposing it outside a trusted network.

### Install (from source / for developers)

```bash
git clone https://github.com/anrted/opendeploy.git
cd opendeploy

# Build binaries
make build

# Install to system using local binaries
sudo make install
```

### Development mode

```bash
# Start Agent (requires root for real operations)
make dev-agent

# Start Core (in another terminal)
make dev-core
```

---

## 📦 Built-in Modules

| Module | Description | Status |
|---|---|---|
| Nginx | Web server & transactional vhost management | Alpha |
| Firewall | UFW-based firewall management | Alpha |
| Certbot | SSL certificate management | Alpha |
| PHP | PHP-FPM package lifecycle foundation | In progress |
| Node.js | Runtime package lifecycle foundation | In progress |
| Git | Package lifecycle foundation | Alpha |
| MySQL | Relational database management | Planned |
| PostgreSQL | Relational database management | Planned |
| Apache | Web server | Planned |
| Fail2ban | Intrusion-prevention jail management | Alpha |
| Cron | Scheduled task management and execution history | Alpha |

---

## 🔧 Configuration

Copy `configs/opendeploy.yaml` to `/etc/opendeploy/opendeploy.yaml` and set:

```yaml
auth:
  jwt_secret: "your-32-char-secret-here"  # REQUIRED in production
```

Or export `OD_JWT_SECRET` environment variable.

See [configs/opendeploy.yaml](configs/opendeploy.yaml) for all options.

---

## 📚 Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) — System design and component interaction
- [API.md](API.md) — REST API reference
- [INSTALL.md](INSTALL.md) — Detailed installation guide
- [TESTING.md](TESTING.md) — How to run tests and CI/CD details
- [SECURITY.md](SECURITY.md) — Security model and reporting
- [UPDATE_SECURITY.md](UPDATE_SECURITY.md) — Signed releases, health checks and rollback
- [UPDATER_AUDIT.md](UPDATER_AUDIT.md) — Updater findings and remediation report
- [CONTRIBUTING.md](CONTRIBUTING.md) — How to contribute
- [ROADMAP.md](ROADMAP.md) — Future plans
- [CHANGELOG.md](CHANGELOG.md) — Version history
- [AUDIT.md](AUDIT.md) — Readiness, security findings and priority backlog
- [PRODUCTION_STAGE1.md](PRODUCTION_STAGE1.md) — First security-foundation remediation report

---

## 🤝 Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a PR.

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.
