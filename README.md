# OpenDeploy

> **Early-alpha, open-source Linux server management platform**

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Architecture](https://img.shields.io/badge/Architecture-Modular-brightgreen)](#architecture)

OpenDeploy is a **modular server management platform** — not just a control panel. It provides a beautiful, modern web interface to fully manage your Linux server without SSH, while every component remains independent and extensible.

---

## ✨ Features

- 🔒 **Secure by design** — Backend never runs as root; all system operations go through an isolated Agent
- 🧩 **Truly modular** — Core knows nothing about Nginx, PHP or Docker; modules register themselves
- ⚡ **Fast & lightweight** — Pure Go backend, Vue 3 + TailwindCSS frontend, embedded as single binary
- 📡 **Real-time** — WebSocket-powered live metrics, job output streaming
- 🔌 **Self-contained** — No Nginx/Apache/Node.js/MySQL dependency; runs on port 5888 out of the box
- 🛡️ **JWT + RBAC** — Access tokens, refresh rotation, role-based permissions (admin/operator/viewer)

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

---

## 🚀 Quick Start

### Requirements

- Linux (Debian/Ubuntu or RHEL/CentOS/Fedora)
- Go 1.23+ (for building from source)

### Install

OpenDeploy can be installed on Ubuntu 22.04+ with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/anrted/opendeploy/main/install.sh | sudo bash
```

After installation, open `http://YOUR_SERVER_IP:5888` in your browser. You will be greeted by the initial setup wizard to create your administrator account and configure the server.

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
| PHP | PHP-FPM package lifecycle foundation | In progress |
| Node.js | Runtime package lifecycle foundation | In progress |
| Git | Package lifecycle foundation | In progress |

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
- [SECURITY.md](SECURITY.md) — Security model and reporting
- [CONTRIBUTING.md](CONTRIBUTING.md) — How to contribute
- [ROADMAP.md](ROADMAP.md) — Future plans
- [CHANGELOG.md](CHANGELOG.md) — Version history

---

## 🤝 Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a PR.

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.
