#!/bin/sh
set -e

# ==============================================================================
# OpenDeploy Development Installation Script
# ==============================================================================
# This script installs OpenDeploy on local development systems.
# It uses locally built binaries instead of downloading from GitHub.
# ==============================================================================

PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PREFIX="/usr/bin"
CONFIG_DIR="/etc/opendeploy"
STATE_DIR="/var/lib/opendeploy"
LOG_DIR="/var/log/opendeploy"
SYSTEMD_DIR="/etc/systemd/system"
GENERATED_ADMIN_PASSWORD=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_step() {
    printf "${BLUE}==>${NC} %s...\n" "$1"
}

print_success() {
    printf "${GREEN}✓${NC} %s\n" "$1"
}

print_error() {
    printf "${RED}✗${NC} %s\n" "$1" >&2
}

print_warning() {
    printf "${YELLOW}!${NC} %s\n" "$1"
}

die() {
    print_error "$1"
    exit 1
}

# 1. Проверить права root
print_step "Проверка прав root"
if [ "$(id -u)" -ne 0 ]; then
    die "Этот скрипт должен выполняться от имени root (sudo)"
fi
print_success "Права root подтверждены"

# 2. Установка бинарных файлов (из локальной сборки)
print_step "Установка локальных бинарных файлов"
if [ ! -f "$PROJECT_DIR/bin/opendeploy-core" ]; then
    die "Бинарные файлы не найдены. Сначала запустите 'make build'"
fi

install -d -m 0755 "$PREFIX"
install -m 0755 "$PROJECT_DIR/bin/opendeploy-core" "$PREFIX/opendeploy-core"
install -m 0755 "$PROJECT_DIR/bin/opendeploy-agent" "$PREFIX/opendeploy-agent"
install -m 0755 "$PROJECT_DIR/bin/opendeploy-cli" "$PREFIX/opendeploy"
print_success "Бинарные файлы установлены в $PREFIX"

# 3. Создание пользователя и директорий
print_step "Настройка системного окружения"
if ! getent group opendeploy >/dev/null 2>&1; then
    groupadd --system opendeploy
fi
if ! id opendeploy >/dev/null 2>&1; then
    useradd --system --gid opendeploy --home-dir "$STATE_DIR" --shell /usr/sbin/nologin opendeploy
fi

install -d -m 0750 -o root -g opendeploy "$CONFIG_DIR"
install -d -m 0750 -o opendeploy -g opendeploy "$STATE_DIR" "$LOG_DIR"
print_success "Пользователь и директории созданы"

# 4. Конфигурация
print_step "Генерация конфигурации"
if [ ! -f "$CONFIG_DIR/opendeploy.yaml" ]; then
    cat << 'EOF' > "$CONFIG_DIR/opendeploy.yaml"
# OpenDeploy Configuration
server:
  host: "0.0.0.0"
  port: 5888
  control_plane_enabled: true
  control_plane_port: 5889
agent:
  socket: "/run/opendeploy-agent/agent.sock"
EOF
fi
chown root:opendeploy "$CONFIG_DIR/opendeploy.yaml"
chmod 0640 "$CONFIG_DIR/opendeploy.yaml"

if [ ! -f "$CONFIG_DIR/env" ]; then
    JWT_SECRET=$(od -An -N48 -tx1 /dev/urandom | tr -d ' \n')
    GENERATED_ADMIN_PASSWORD=$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')
    umask 0077
    printf 'OD_JWT_SECRET=%s\nOD_ADMIN_PASSWORD=%s\n' "$JWT_SECRET" "$GENERATED_ADMIN_PASSWORD" > "$CONFIG_DIR/env"
    chown root:opendeploy "$CONFIG_DIR/env"
    chmod 0640 "$CONFIG_DIR/env"
    print_success "Сгенерирован JWT ключ"
elif [ ! -f "$STATE_DIR/data.db" ] && ! grep -q '^OD_ADMIN_PASSWORD=' "$CONFIG_DIR/env"; then
    GENERATED_ADMIN_PASSWORD=$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')
    printf 'OD_ADMIN_PASSWORD=%s\n' "$GENERATED_ADMIN_PASSWORD" >> "$CONFIG_DIR/env"
    chown root:opendeploy "$CONFIG_DIR/env"
    chmod 0640 "$CONFIG_DIR/env"
    print_success "Generated initial administrator password"
else
    print_success "Файл окружения уже существует"
fi

# 5. Создание systemd сервисов
print_step "Настройка systemd сервисов"

printf '%s\n' "$PROJECT_DIR" > "$CONFIG_DIR/source-dir"
chown root:opendeploy "$CONFIG_DIR/source-dir"
chmod 0640 "$CONFIG_DIR/source-dir"

cat << 'EOF' > "$SYSTEMD_DIR/opendeploy-core.service"
[Unit]
Description=OpenDeploy Core — Server Management Panel
Documentation=https://github.com/anrted/opendeploy
After=network.target opendeploy-agent.service
Requires=opendeploy-agent.service

[Service]
Type=simple
User=opendeploy
Group=opendeploy
ExecStart=/usr/bin/opendeploy-core --config /etc/opendeploy/opendeploy.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s
TimeoutStopSec=30s

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=/var/lib/opendeploy /var/log/opendeploy
PrivateTmp=yes
ProtectHome=yes

# Environment
Environment="OD_JWT_SECRET="
EnvironmentFile=-/etc/opendeploy/env

[Install]
WantedBy=multi-user.target
EOF

cat << 'EOF' > "$SYSTEMD_DIR/opendeploy-agent.service"
[Unit]
Description=OpenDeploy Agent — Privileged System Operations
Documentation=https://github.com/anrted/opendeploy
After=network.target

[Service]
Type=simple
User=root
Group=opendeploy
ExecStart=/usr/bin/opendeploy-agent --config /etc/opendeploy/opendeploy.yaml
Restart=on-failure
RestartSec=5s
TimeoutStopSec=30s

# The agent requires root to manage packages, systemd, and firewall.
RuntimeDirectory=opendeploy-agent
RuntimeDirectoryMode=0750
PrivateTmp=yes
ProtectHome=read-only

[Install]
WantedBy=multi-user.target
EOF

cat << 'EOF' > "$SYSTEMD_DIR/opendeploy-update.path"
[Unit]
Description=OpenDeploy Update Watcher
Documentation=https://github.com/anrted/opendeploy

[Path]
PathModified=/var/lib/opendeploy/update.request
Unit=opendeploy-update.service

[Install]
WantedBy=multi-user.target
EOF

cat << 'EOF' > "$SYSTEMD_DIR/opendeploy-update.service"
[Unit]
Description=OpenDeploy Updater
Documentation=https://github.com/anrted/opendeploy

[Service]
Type=oneshot
User=root
ExecStart=/usr/bin/opendeploy update --apply
EOF

systemctl daemon-reload
print_success "Сервисы systemd созданы"

# 6. Запуск сервисов
print_step "Запуск сервисов"
systemctl enable opendeploy-agent.service opendeploy-core.service opendeploy-update.path >/dev/null 2>&1
systemctl restart opendeploy-agent.service opendeploy-core.service opendeploy-update.path
systemctl is-active --quiet opendeploy-agent.service || print_error "Не удалось запустить opendeploy-agent.service"
systemctl is-active --quiet opendeploy-core.service || print_error "Не удалось запустить opendeploy-core.service"
print_success "Сервисы запущены"

echo ""
if [ -n "$GENERATED_ADMIN_PASSWORD" ]; then
    echo "Administrator username: admin"
    printf "Administrator password: %s\n" "$GENERATED_ADMIN_PASSWORD"
    echo "Save this password now. It will not be displayed again."
fi
echo "Password recovery: sudo opendeploy admin reset-password"
printf "${GREEN}Разработческая версия OpenDeploy успешно установлена!${NC}\n"
