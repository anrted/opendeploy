#!/bin/sh
set -eu

missing_packages=""

require_command() {
    command_name=$1
    package_name=$2
    if ! command -v "$command_name" >/dev/null 2>&1; then
        missing_packages="$missing_packages $package_name"
    fi
}

require_command go golang-go
require_command node nodejs
require_command npm npm
require_command cc build-essential
require_command git git

if [ -z "$missing_packages" ]; then
    echo "Build dependencies are available."
    exit 0
fi

if [ "$(uname -s)" != "Linux" ] || ! command -v apt-get >/dev/null 2>&1; then
    echo "Missing build commands:$missing_packages" >&2
    echo "Automatic installation is supported on Debian and Ubuntu only." >&2
    exit 1
fi

if [ "$(id -u)" -eq 0 ]; then
    elevate=""
elif command -v sudo >/dev/null 2>&1; then
    elevate="sudo"
else
    echo "Missing build commands:$missing_packages" >&2
    echo "Install the corresponding packages as root or install sudo." >&2
    exit 1
fi

echo "Installing missing build packages:$missing_packages"
$elevate apt-get update
# shellcheck disable=SC2086
$elevate apt-get install -y $missing_packages

for command_name in go node npm cc git; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "Dependency installation completed, but '$command_name' is still unavailable." >&2
        exit 1
    fi
done

echo "Build dependencies installed."
