#!/bin/sh
set -eu

PREFIX="${PREFIX:-/usr/local}"
CONFIG_DIR="${CONFIG_DIR:-/etc/opendeploy}"
STATE_DIR="${STATE_DIR:-/var/lib/opendeploy}"
LOG_DIR="${LOG_DIR:-/var/log/opendeploy}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
initial_password=""

[ "$(id -u)" -eq 0 ] || { echo "install.sh must run as root" >&2; exit 1; }
command -v systemctl >/dev/null 2>&1 || { echo "systemd is required" >&2; exit 1; }

getent group opendeploy >/dev/null 2>&1 || groupadd --system opendeploy
id opendeploy >/dev/null 2>&1 || useradd --system --gid opendeploy \
    --home-dir "$STATE_DIR" --shell /usr/sbin/nologin opendeploy

install -d -m 0750 -o root -g opendeploy "$CONFIG_DIR"
install -d -m 0750 -o opendeploy -g opendeploy "$STATE_DIR" "$LOG_DIR"
install -d -m 0755 "$PREFIX/bin"
install -d -m 0755 "$PREFIX/lib/opendeploy"
install -m 0755 "$PROJECT_DIR/bin/opendeploy-core" "$PREFIX/bin/opendeploy-core"
install -m 0755 "$PROJECT_DIR/bin/opendeploy-agent" "$PREFIX/bin/opendeploy-agent"
install -m 0755 "$PROJECT_DIR/bin/opendeploy-cli" "$PREFIX/bin/opendeploy"
install -m 0644 "$PROJECT_DIR/deployments/systemd/opendeploy-core.service" "$SYSTEMD_DIR/opendeploy-core.service"
install -m 0644 "$PROJECT_DIR/deployments/systemd/opendeploy-agent.service" "$SYSTEMD_DIR/opendeploy-agent.service"
install -m 0644 "$PROJECT_DIR/deployments/systemd/opendeploy-update.service" "$SYSTEMD_DIR/opendeploy-update.service"
install -m 0644 "$PROJECT_DIR/deployments/systemd/opendeploy-update.path" "$SYSTEMD_DIR/opendeploy-update.path"
install -m 0755 "$PROJECT_DIR/deployments/update.sh" "$PREFIX/lib/opendeploy/update.sh"
printf '%s\n' "$PROJECT_DIR" >"$CONFIG_DIR/source-dir"
chown root:opendeploy "$CONFIG_DIR/source-dir"
chmod 0640 "$CONFIG_DIR/source-dir"

if [ ! -f "$CONFIG_DIR/opendeploy.yaml" ]; then
    install -m 0640 -o root -g opendeploy "$PROJECT_DIR/configs/opendeploy.yaml" "$CONFIG_DIR/opendeploy.yaml"
fi
chown root:opendeploy "$CONFIG_DIR/opendeploy.yaml"
chmod 0640 "$CONFIG_DIR/opendeploy.yaml"
if [ ! -f "$CONFIG_DIR/env" ]; then
    jwt_secret=$(od -An -N48 -tx1 /dev/urandom | tr -d ' \n')
    admin_password=$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')
    initial_password=$admin_password
    umask 0077
    printf 'OD_JWT_SECRET=%s\nOD_ADMIN_PASSWORD=%s\n' "$jwt_secret" "$admin_password" >"$CONFIG_DIR/env"
    chown root:opendeploy "$CONFIG_DIR/env"
    chmod 0640 "$CONFIG_DIR/env"
fi

systemctl daemon-reload
systemctl enable opendeploy-agent.service opendeploy-core.service
systemctl enable --now opendeploy-update.path
systemctl restart opendeploy-agent.service
systemctl restart opendeploy-core.service
systemctl is-active --quiet opendeploy-agent.service
systemctl is-active --quiet opendeploy-core.service
systemctl is-active --quiet opendeploy-update.path

panel_host=$(hostname -I 2>/dev/null | awk '{print $1}')
if [ -z "$panel_host" ]; then
    panel_host="SERVER_IP"
fi

echo ""
echo "OpenDeploy installed and started successfully."
printf 'Web panel: http://%s:5888\n' "$panel_host"
echo "Username: admin"
if [ -n "$initial_password" ]; then
    printf 'Password: %s\n' "$initial_password"
    echo "Store this password securely. It is shown only during initial installation."
else
    echo "Password: existing administrator password (not displayed again)"
fi
