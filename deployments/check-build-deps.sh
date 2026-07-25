#!/bin/sh
set -eu

required_node_major=22
required_node_minor=12
missing_packages=""
node_upgrade=0

require_command() {
    command_name=$1
    package_name=$2
    if ! command -v "$command_name" >/dev/null 2>&1; then
        missing_packages="$missing_packages $package_name"
    fi
}

node_is_supported() {
    command -v node >/dev/null 2>&1 || return 1
    version=$(node -p "process.versions.node" 2>/dev/null) || return 1
    major=${version%%.*}
    remainder=${version#*.}
    minor=${remainder%%.*}
    case "$major:$minor" in
        *[!0-9:]*|:*) return 1 ;;
    esac
    [ "$major" -gt "$required_node_major" ] ||
        { [ "$major" -eq "$required_node_major" ] && [ "$minor" -ge "$required_node_minor" ]; }
}

require_command go golang-go
require_command cc build-essential
require_command git git

if ! node_is_supported || ! command -v npm >/dev/null 2>&1; then
    node_upgrade=1
fi

if [ -z "$missing_packages" ] && [ "$node_upgrade" -eq 0 ]; then
    echo "Build dependencies are available."
    exit 0
fi

if [ "$(uname -s)" != "Linux" ] || ! command -v apt-get >/dev/null 2>&1; then
    [ -z "$missing_packages" ] || echo "Missing build packages:$missing_packages" >&2
    [ "$node_upgrade" -eq 0 ] ||
        echo "Node.js ${required_node_major}.${required_node_minor}+ and npm are required." >&2
    echo "Automatic installation is supported on Debian and Ubuntu only." >&2
    exit 1
fi

if [ "$(id -u)" -eq 0 ]; then
    elevate=""
elif command -v sudo >/dev/null 2>&1; then
    elevate="sudo"
else
    echo "Build dependency installation requires root or sudo." >&2
    exit 1
fi

if [ -n "$missing_packages" ]; then
    echo "Installing missing build packages:$missing_packages"
    $elevate apt-get update
    # shellcheck disable=SC2086
    $elevate apt-get install -y $missing_packages
fi

if [ "$node_upgrade" -eq 1 ]; then
    echo "Installing Node.js 22.x and npm from the signed NodeSource APT repository."
    $elevate apt-get update
    $elevate apt-get install -y ca-certificates curl gnupg

    architecture=$(dpkg --print-architecture)
    case "$architecture" in
        amd64|arm64) ;;
        *)
            echo "NodeSource supports only amd64 and arm64 for this installer; found $architecture." >&2
            exit 1
            ;;
    esac

    key_file=$(mktemp)
    trap 'rm -f "$key_file"' EXIT HUP INT TERM
    curl -fsSL -o "$key_file" https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key
    $elevate install -d -m 0755 /usr/share/keyrings
    $elevate gpg --dearmor --yes -o /usr/share/keyrings/nodesource.gpg "$key_file"
    $elevate chmod 0644 /usr/share/keyrings/nodesource.gpg

    printf '%s\n' \
        "Types: deb" \
        "URIs: https://deb.nodesource.com/node_22.x" \
        "Suites: nodistro" \
        "Components: main" \
        "Architectures: $architecture" \
        "Signed-By: /usr/share/keyrings/nodesource.gpg" |
        $elevate tee /etc/apt/sources.list.d/nodesource.sources >/dev/null
    printf '%s\n' \
        "Package: nodejs" \
        "Pin: origin deb.nodesource.com" \
        "Pin-Priority: 600" |
        $elevate tee /etc/apt/preferences.d/nodejs >/dev/null

    $elevate apt-get update
    $elevate apt-get install -y nodejs
fi

for command_name in go node npm cc git; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "Dependency installation completed, but '$command_name' is still unavailable." >&2
        exit 1
    fi
done

if ! node_is_supported; then
    echo "Node.js $(node --version) is too old; ${required_node_major}.${required_node_minor}+ is required." >&2
    exit 1
fi

echo "Build dependencies installed."
