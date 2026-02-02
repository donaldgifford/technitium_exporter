#!/bin/sh
set -e

# Create technitium_exporter user and group if they don't exist
if ! getent group technitium_exporter >/dev/null 2>&1; then
    groupadd --system technitium_exporter
fi

if ! getent passwd technitium_exporter >/dev/null 2>&1; then
    useradd --system --no-create-home --shell /usr/sbin/nologin \
        --gid technitium_exporter technitium_exporter
fi

# Reload systemd to pick up the new service file
systemctl daemon-reload

# Enable the service (but don't start - user needs to configure first)
systemctl enable technitium_exporter.service

echo ""
echo "technitium-exporter has been installed."
echo ""
echo "Before starting the service, configure your Technitium server details:"
echo "  sudo nano /etc/default/technitium_exporter"
echo ""
echo "Then start the service:"
echo "  sudo systemctl start technitium_exporter"
echo ""
