#!/bin/sh
set -eu

[ "$(id -u)" -eq 0 ] || { echo "uninstall.sh must run as root" >&2; exit 1; }
case "${1:-}" in "" ) purge=0 ;; --purge ) purge=1 ;; * ) echo "usage: $0 [--purge]" >&2; exit 2 ;; esac

systemctl disable --now opendeploy-core.service opendeploy-agent.service 2>/dev/null || true
rm -f /etc/systemd/system/opendeploy-core.service /etc/systemd/system/opendeploy-agent.service
rm -f /usr/local/bin/opendeploy-core /usr/local/bin/opendeploy-agent /usr/local/bin/opendeploy
systemctl daemon-reload

if [ "$purge" -eq 1 ]; then
    rm -rf /etc/opendeploy /var/lib/opendeploy /var/log/opendeploy
    userdel opendeploy 2>/dev/null || true
    groupdel opendeploy 2>/dev/null || true
    echo "OpenDeploy uninstalled and data purged."
else
    echo "OpenDeploy uninstalled. Configuration and data were preserved."
fi
