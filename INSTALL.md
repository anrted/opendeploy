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

`make build` only compiles the project. `sudo make install` installs and starts
the systemd services, then prints the panel URL and initial administrator
credentials.

`make build` checks the required commands and their supported versions first.
On Ubuntu and Debian it uses APT (through `sudo` when needed) to install
missing packages. Node.js older than 22.12 is upgraded to Node.js 22 from the
signed NodeSource APT repository. On other distributions the check reports
what must be upgraded without modifying the system.

The installer creates the locked `opendeploy` service account, secures the
Agent Unix socket through the `opendeploy` group, generates both required
secrets, enables and starts Agent before Core, and prints the one-time initial
administrator password. Store it securely.

The initial username is `admin`. Remove `OD_ADMIN_PASSWORD` from the environment
file after the first successful startup. Place OpenDeploy behind an HTTPS
reverse proxy before exposing it outside a trusted network.

The installer records the canonical source checkout in
`/etc/opendeploy/source-dir` and enables `opendeploy-update.path`. Administrators
can then apply a clean fast-forward update from the trusted GitHub repository
in Settings. Existing credentials, configuration, and SQLite data are
preserved.

To uninstall while preserving configuration and data:

```bash
sudo sh deployments/uninstall.sh
```

Use `--purge` only when all OpenDeploy configuration and data should be
permanently removed.
