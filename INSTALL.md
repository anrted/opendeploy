# Installation

OpenDeploy is early alpha software. Use it on a test server, not a production
host.

## Build requirements

- Linux
- Go 1.23+
- Node.js 22+ and npm
- GCC and SQLite development support for CGO
- systemd

```bash
git clone https://github.com/anrted/opendeploy.git
cd opendeploy

cd web
npm ci
npm run build
cd ..

make build
sudo make install
```

Create `/etc/opendeploy/env` readable only by root and the `opendeploy` group:

```bash
OD_JWT_SECRET=<random value of at least 32 bytes>
OD_ADMIN_PASSWORD=<unique initial password of at least 12 characters>
```

Then start Agent before Core:

```bash
sudo systemctl enable --now opendeploy-agent
sudo systemctl enable --now opendeploy-core
```

The initial username is `admin`. Remove `OD_ADMIN_PASSWORD` from the environment
file after the first successful startup. Place OpenDeploy behind an HTTPS
reverse proxy before exposing it outside a trusted network.
