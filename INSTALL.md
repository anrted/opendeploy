# Installation

OpenDeploy is early alpha software. Use it on a test server, not a production
host.

## Quick Installation (Ubuntu 22.04+)

The easiest way to install OpenDeploy is using the automated installation script. It downloads the pre-compiled binaries, creates the necessary users and directories, sets up systemd services, and generates the initial JWT secret.

```bash
curl -fsSL https://raw.githubusercontent.com/anrted/opendeploy/main/install.sh | sudo bash
```

To install the latest development (nightly) build instead of the stable release:
```bash
curl -fsSL https://raw.githubusercontent.com/anrted/opendeploy/main/install.sh | sudo bash -s -- --dev
```

## Build from Source

If you prefer to compile from source, you need Go 1.23+ and Node.js.

```bash
git clone https://github.com/anrted/opendeploy.git
cd opendeploy

make build
sudo make install
```

`make build` compiles the Go binaries and the Vue 3 frontend. `sudo make install` installs the binaries, configures the systemd services, and starts the panel.

## Initial Setup

After installation, access the panel in your browser:
```
http://<YOUR_SERVER_IP>:5888
```

You will be greeted by the initial setup wizard to create your administrator account.

## Updates

OpenDeploy includes an automated updater. You can apply stable or development updates directly from the Settings page in the web interface.

## Uninstallation

To uninstall OpenDeploy:

```bash
sudo sh deployments/uninstall.sh
```

Use `--purge` only when all OpenDeploy configuration and data should be permanently removed.
