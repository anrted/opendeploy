#!/bin/sh
set -eu

REPOSITORY="anrted/opendeploy"
CONFIG_DIR="/etc/opendeploy"
STATE_DIR="/var/lib/opendeploy-agent"
SYSTEMD_DIR="/etc/systemd/system"
CORE_URL=""
REGISTRATION_TOKEN=""

step() { printf '\033[1;34m==>\033[0m %s\n' "$1"; }
ok() { printf '\033[1;32m✓\033[0m %s\n' "$1"; }
die() { printf '\033[1;31m✗\033[0m %s\n' "$1" >&2; exit 1; }

while [ "$#" -gt 0 ]; do
    case "$1" in
        --server) [ "$#" -ge 2 ] || die "--server requires a URL"; CORE_URL=$2; shift 2 ;;
        --token) [ "$#" -ge 2 ] || die "--token requires a value"; REGISTRATION_TOKEN=$2; shift 2 ;;
        *) die "unknown argument: $1" ;;
    esac
done

[ "$(id -u)" -eq 0 ] || die "run this installer as root"
[ -n "$CORE_URL" ] || die "--server is required"
[ -n "$REGISTRATION_TOKEN" ] || die "--token is required"
case "$CORE_URL" in
    https://*) ;;
    http://*) printf '\033[1;33m!\033[0m %s\n' "Core URL uses unencrypted HTTP; enrollment tokens and agent traffic can be intercepted" >&2 ;;
    *) die "Core URL must start with http:// or https://" ;;
esac

step "Detect OS"
[ -r /etc/os-release ] || die "unsupported OS: /etc/os-release is missing"
. /etc/os-release
case "${ID:-}" in ubuntu|debian) ;; *) die "supported distributions: Ubuntu and Debian" ;; esac
command -v systemctl >/dev/null 2>&1 || die "systemd is required"
ok "Detect OS: ${PRETTY_NAME:-$ID}"

step "Detect Architecture"
case "$(uname -m)" in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; *) die "unsupported architecture" ;; esac
ok "Detect Architecture: $ARCH"

step "Check Root"
ok "Check Root"

AVAILABLE_KB=$(df -Pk /usr/local/bin | awk 'NR==2 {print $4}')
[ "${AVAILABLE_KB:-0}" -ge 102400 ] || die "at least 100 MB free disk space is required"
RAM_KB=$(awk '/MemTotal/ {print $2}' /proc/meminfo)
[ "${RAM_KB:-0}" -ge 131072 ] || die "at least 128 MB RAM is required"
for tool in curl openssl sha256sum tar awk sed uname hostname; do command -v "$tool" >/dev/null 2>&1 || die "$tool is required"; done
if ! command -v jq >/dev/null 2>&1; then
    apt-get update
    apt-get install -y jq
fi

step "Install Agent"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT
RELEASE_JSON=$(curl -fsSL "https://api.github.com/repos/$REPOSITORY/releases/latest")
TAG=$(printf '%s' "$RELEASE_JSON" | jq -r .tag_name)
ARCHIVE="opendeploy-linux-$ARCH.tar.gz"
curl -fsSL "https://github.com/$REPOSITORY/releases/download/$TAG/$ARCHIVE" -o "$TMP_DIR/$ARCHIVE"
curl -fsSL "https://github.com/$REPOSITORY/releases/download/$TAG/checksums.txt" -o "$TMP_DIR/checksums.txt"
(cd "$TMP_DIR" && grep " $ARCHIVE\$" checksums.txt | sha256sum -c -) || die "release checksum verification failed"
tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR" opendeploy-agent
install -m 0755 "$TMP_DIR/opendeploy-agent" /usr/local/bin/opendeploy-agent
ok "Install Agent: $TAG"

install -d -m 0700 "$CONFIG_DIR" "$STATE_DIR"
MACHINE_ID=$(cat /etc/machine-id 2>/dev/null || openssl rand -hex 16)
PRIVATE_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
KERNEL=$(uname -r)
CPU_MODEL=$(awk -F: '/model name/ {gsub(/^ /,"",$2); print $2; exit}' /proc/cpuinfo)
CPU_CORES=$(getconf _NPROCESSORS_ONLN)
RAM_TOTAL=$((RAM_KB * 1024))
DISK_TOTAL=$(df -PB1 / | awk 'NR==2 {print $2}')

step "Register Server"
openssl ecparam -genkey -name prime256v1 -out "$CONFIG_DIR/agent.key"
openssl req -new -key "$CONFIG_DIR/agent.key" -subj "/CN=$(hostname)/O=OpenDeploy Agents" -out "$TMP_DIR/agent.csr"
REGISTER_PAYLOAD=$(jq -n \
  --arg token "$REGISTRATION_TOKEN" --arg machine_id "$MACHINE_ID" --arg hostname "$(hostname)" \
  --arg os "$ID" --arg distribution "${NAME:-$ID}" --arg os_version "${VERSION_ID:-}" \
  --arg kernel "$KERNEL" --arg architecture "$ARCH" --arg cpu_model "$CPU_MODEL" \
  --arg private_ip "${PRIVATE_IP:-}" --arg agent_version "$TAG" --rawfile csr "$TMP_DIR/agent.csr" \
  --argjson cpu_cores "$CPU_CORES" --argjson ram_total "$RAM_TOTAL" --argjson disk_total "$DISK_TOTAL" \
  '{token:$token,machine_id:$machine_id,hostname:$hostname,os:$os,distribution:$distribution,
    os_version:$os_version,kernel:$kernel,architecture:$architecture,cpu_model:$cpu_model,
    cpu_cores:$cpu_cores,ram_total:$ram_total,disk_total:$disk_total,private_ip:$private_ip,
    agent_version:$agent_version,csr:$csr}')
REGISTER_RESPONSE=$(curl -fsS -H 'Content-Type: application/json' -d "$REGISTER_PAYLOAD" \
  "$CORE_URL/api/v1/agents/register") || die "registration failed"
SERVER_ID=$(printf '%s' "$REGISTER_RESPONSE" | jq -er .server_id)
FINGERPRINT=$(printf '%s' "$REGISTER_RESPONSE" | jq -er .fingerprint)
CONTROL_PLANE_ADDRESS=$(printf '%s' "$REGISTER_RESPONSE" | jq -r '.control_plane_address // empty')
printf '%s' "$REGISTER_RESPONSE" | jq -er .certificate > "$CONFIG_DIR/agent.crt"
if [ -n "$CONTROL_PLANE_ADDRESS" ]; then
  printf '%s' "$REGISTER_RESPONSE" | jq -er .control_plane_ca > "$CONFIG_DIR/control-plane-ca.crt"
fi
chmod 0600 "$CONFIG_DIR/agent.crt" "$CONFIG_DIR/agent.key"
if [ -f "$CONFIG_DIR/control-plane-ca.crt" ]; then
  chmod 0600 "$CONFIG_DIR/control-plane-ca.crt"
fi
ok "Register Server: $SERVER_ID"

step "Configure TLS"
openssl x509 -in "$CONFIG_DIR/agent.crt" -noout -checkend 86400 >/dev/null || die "issued certificate is invalid"
ok "Configure TLS"

cat > "$CONFIG_DIR/agent.yaml" <<EOF
agent:
  socket: "/run/opendeploy-agent/agent.sock"
  timeout: 120s
  core_url: "$CORE_URL"
  server_id: "$SERVER_ID"
  certificate_file: "$CONFIG_DIR/agent.crt"
  private_key_file: "$CONFIG_DIR/agent.key"
  certificate_fingerprint: "$FINGERPRINT"
  heartbeat_interval: 30s
$(if [ -n "$CONTROL_PLANE_ADDRESS" ]; then
    printf '  control_plane_address: "%s"\n  control_plane_ca_file: "%s/control-plane-ca.crt"\n' "$CONTROL_PLANE_ADDRESS" "$CONFIG_DIR"
  fi)
logging:
  level: "info"
  format: "json"
EOF
chmod 0600 "$CONFIG_DIR/agent.yaml"

cat > "$SYSTEMD_DIR/opendeploy-agent.service" <<'EOF'
[Unit]
Description=OpenDeploy Remote Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/opendeploy-agent --config /etc/opendeploy/agent.yaml
Restart=always
RestartSec=5s
RuntimeDirectory=opendeploy-agent
RuntimeDirectoryMode=0750
NoNewPrivileges=yes
PrivateTmp=yes
ProtectHome=read-only

[Install]
WantedBy=multi-user.target
EOF

step "Start Agent"
systemctl daemon-reload
systemctl enable --now opendeploy-agent.service
systemctl is-active --quiet opendeploy-agent.service || die "agent service did not start"
ok "Start Agent"

step "Health Check"
sleep 2
systemctl is-active --quiet opendeploy-agent.service || die "agent health check failed"
ok "Health Check"
ok "Done"
