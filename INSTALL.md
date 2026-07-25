# Installation

OpenDeploy is early alpha software. Use it on a test server, not a production
host.

## Build requirements

- Linux
- Go 1.25+
- Node.js 22+ and npm
- GCC and SQLite development support for CGO
- systemd

```bash
git clone https://github.com/anrted/opendeploy.git
cd opendeploy

make build
sudo make install
```

`make build` checks the required commands first. On Ubuntu and Debian it uses
APT (through `sudo` when needed) to install missing Go, Node.js/npm, compiler,
and Git packages. On other distributions it reports the missing commands
without modifying the system.

The installer creates the locked `opendeploy` service account, secures the
Agent Unix socket through the `opendeploy` group, generates both required
secrets, enables and starts Agent before Core, and prints the one-time initial
administrator password. Store it securely.

The initial username is `admin`. Remove `OD_ADMIN_PASSWORD` from the environment
file after the first successful startup. Place OpenDeploy behind an HTTPS
reverse proxy before exposing it outside a trusted network.

To uninstall while preserving configuration and data:

```bash
sudo sh deployments/uninstall.sh
```

Use `--purge` only when all OpenDeploy configuration and data should be
permanently removed.
