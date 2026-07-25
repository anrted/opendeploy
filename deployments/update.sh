#!/bin/sh
set -eu

request_file="/var/lib/opendeploy/update.request"
source_file="/etc/opendeploy/source-dir"
rm -f "$request_file"

[ "$(id -u)" -eq 0 ] || { echo "OpenDeploy updater must run as root" >&2; exit 1; }
[ -f "$source_file" ] || { echo "OpenDeploy source path is not configured" >&2; exit 1; }

source_dir=$(sed -n '1p' "$source_file")
case "$source_dir" in
    /*) ;;
    *) echo "Invalid OpenDeploy source path" >&2; exit 1 ;;
esac
[ -d "$source_dir/.git" ] || { echo "OpenDeploy source repository not found: $source_dir" >&2; exit 1; }
branch=$(git -C "$source_dir" rev-parse --abbrev-ref HEAD)
[ "$branch" = "main" ] || { echo "Refusing update from branch: $branch" >&2; exit 1; }

remote=$(git -C "$source_dir" remote get-url origin)
case "$remote" in
    https://github.com/anrted/opendeploy|https://github.com/anrted/opendeploy.git|git@github.com:anrted/opendeploy.git) ;;
    *) echo "Refusing update from untrusted remote: $remote" >&2; exit 1 ;;
esac

if [ -n "$(git -C "$source_dir" status --porcelain -uno)" ]; then
    echo "Refusing update because the source repository has local changes" >&2
    exit 1
fi

git -C "$source_dir" fetch --tags origin main
git -C "$source_dir" merge --ff-only origin/main
make -C "$source_dir" build
sh "$source_dir/deployments/install.sh"
