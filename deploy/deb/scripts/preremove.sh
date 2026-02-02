#!/bin/sh
set -e

# Stop and disable the service if it's running
if systemctl is-active --quiet technitium_exporter.service; then
    systemctl stop technitium_exporter.service
fi

if systemctl is-enabled --quiet technitium_exporter.service 2>/dev/null; then
    systemctl disable technitium_exporter.service
fi

systemctl daemon-reload
