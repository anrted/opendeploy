#!/bin/sh
set -e

# ==============================================================================
# OpenDeploy Installation Script
# ==============================================================================
# This script installs OpenDeploy on Ubuntu systems.
# It downloads the latest binaries from GitHub, configures users, directories,
# systemd services, and generates initial secrets.
# ==============================================================================

REPO="anrted/opendeploy"
GITHUB_API="https://api.github.com/repos/$REPO/releases/latest"

PREFIX="/usr/bin"
CONFIG_DIR="/etc/opendeploy"
STATE_DIR="/var/lib/opendeploy"
LOG_DIR="/var/log/opendeploy"
SYSTEMD_DIR="/etc/systemd/system"

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

# 2. Определить архитектуру
print_step "Определение архитектуры"
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) die "Неподдерживаемая архитектура: $ARCH" ;;
esac
print_success "Архитектура: $ARCH"

# 3. Проверить ОС (Ubuntu)
print_step "Проверка ОС"
if [ -f /etc/os-release ]; then
    . /etc/os-release
    if [ "$ID" != "ubuntu" ]; then
        print_warning "Эта система не определена как Ubuntu. Возможны проблемы с совместимостью."
    else
        print_success "ОС: Ubuntu $VERSION_ID"
    fi
else
    print_warning "Файл /etc/os-release не найден. Невозможно определить ОС."
fi

# 4. Проверить systemd
print_step "Проверка systemd"
if ! command -v systemctl >/dev/null 2>&1; then
    die "systemd не установлен, но требуется для работы OpenDeploy"
fi
print_success "systemd найден"

# 5. Проверить утилиты
print_step "Проверка системных утилит"
for req in curl tar jq sha256sum grep awk; do
    if ! command -v "$req" >/dev/null 2>&1; then
        print_step "Установка недостающей утилиты: $req"
        apt-get update -qq && apt-get install -y -qq "$req" >/dev/null 2>&1 || die "Не удалось установить $req"
    fi
done
print_success "Все необходимые утилиты установлены"

USE_DEV=0
for arg in "$@"; do
    case $arg in
        --dev|--nightly)
            USE_DEV=1
            shift
            ;;
    esac
done

if [ "$USE_DEV" -eq 1 ]; then
    print_step "Использование тестовой (nightly) сборки"
    VERSION="nightly"
else
    # 6. Получение последнего релиза
    print_step "Поиск последней стабильной версии"
    LATEST_RELEASE=$(curl -fsSL "$GITHUB_API" || echo "")
    VERSION=$(echo "$LATEST_RELEASE" | jq -r .tag_name || echo "null")
    if [ "$VERSION" = "null" ] || [ -z "$VERSION" ]; then
        die "Не удалось определить последнюю версию релиза. Проверьте подключение к GitHub."
    fi
    print_success "Найдена версия: $VERSION"
fi

TAR_NAME="opendeploy-linux-${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${TAR_NAME}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# 7. Скачивание
print_step "Скачивание OpenDeploy $VERSION"
curl -fsSL --progress-bar "$DOWNLOAD_URL" -o "$TMP_DIR/$TAR_NAME"
curl -fsSL "$CHECKSUMS_URL" -o "$TMP_DIR/checksums.txt"
print_success "Скачивание завершено"

# 8. Проверка контрольной суммы
print_step "Проверка контрольной суммы"
cd "$TMP_DIR"
if ! grep "$TAR_NAME" checksums.txt | sha256sum -c - >/dev/null 2>&1; then
    die "Проверка контрольной суммы провалилась!"
fi
print_success "Контрольная сумма верна"

# 9. Распаковка и установка
print_step "Установка бинарных файлов"
tar -xzf "$TAR_NAME"
install -d -m 0755 "$PREFIX"
install -m 0755 opendeploy-core "$PREFIX/opendeploy-core"
install -m 0755 opendeploy-agent "$PREFIX/opendeploy-agent"
install -m 0755 opendeploy-cli "$PREFIX/opendeploy"
print_success "Бинарные файлы установлены в $PREFIX"

# 10. Создание пользователя и директорий
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

# 11. Конфигурация
print_step "Генерация конфигурации"
if [ ! -f "$CONFIG_DIR/opendeploy.yaml" ]; then
    cat << 'EOF' > "$CONFIG_DIR/opendeploy.yaml"
# OpenDeploy Configuration
server:
  addr: :5888
  debug: false
agent:
  socket_path: /run/opendeploy-agent/agent.sock
EOF
fi
chown root:opendeploy "$CONFIG_DIR/opendeploy.yaml"
chmod 0640 "$CONFIG_DIR/opendeploy.yaml"

if [ ! -f "$CONFIG_DIR/env" ]; then
    JWT_SECRET=$(od -An -N48 -tx1 /dev/urandom | tr -d ' \n')
    umask 0077
    printf 'OD_JWT_SECRET=%s\n' "$JWT_SECRET" > "$CONFIG_DIR/env"
    chown root:opendeploy "$CONFIG_DIR/env"
    chmod 0640 "$CONFIG_DIR/env"
    print_success "Сгенерирован JWT ключ"
else
    print_success "Файл окружения уже существует"
fi

# 12. Создание systemd сервисов
print_step "Настройка systemd сервисов"

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
PathModified=/etc/opendeploy/update-trigger
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

# 13. Запуск сервисов
print_step "Запуск сервисов"
systemctl enable opendeploy-agent.service opendeploy-core.service opendeploy-update.path >/dev/null 2>&1
systemctl restart opendeploy-agent.service opendeploy-core.service opendeploy-update.path
systemctl is-active --quiet opendeploy-agent.service || print_error "Не удалось запустить opendeploy-agent.service"
systemctl is-active --quiet opendeploy-core.service || print_error "Не удалось запустить opendeploy-core.service"
print_success "Сервисы запущены"

# 14. Настройка UFW
print_step "Настройка брандмауэра (UFW)"
if command -v ufw >/dev/null 2>&1 && ufw status | grep -q "Status: active"; then
    ufw allow 5888/tcp comment 'OpenDeploy Panel' >/dev/null 2>&1
    print_success "Порт 5888 открыт в UFW"
else
    print_warning "UFW не активен или не установлен. Пропущено."
fi

# 15. Вывод информации
PANEL_HOST=$(hostname -I 2>/dev/null | awk '{print $1}')
if [ -z "$PANEL_HOST" ]; then
    PANEL_HOST="SERVER_IP"
fi

echo ""
echo "==================================================================="
printf "${GREEN}OpenDeploy успешно установлен и запущен!${NC}\n"
echo "==================================================================="
printf "🌐 URL панели:       http://%s:5888\n" "$PANEL_HOST"
printf "🛠  Конфигурация:     %s\n" "$CONFIG_DIR"
printf "📝 Логи:             %s\n" "$LOG_DIR"
echo ""
echo "Команды управления:"
echo "  systemctl status opendeploy-core"
echo "  systemctl restart opendeploy-core"
echo "  systemctl restart opendeploy-agent"
echo "==================================================================="
echo "⚠️  Важно: Перейдите в панель управления для первоначальной настройки"
echo "   системы и создания учетной записи администратора."
echo "==================================================================="
echo ""
