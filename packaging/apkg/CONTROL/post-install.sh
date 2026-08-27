#!/bin/sh
set -u
PKG_PATH="/usr/local/AppCentral/CronManager"
mkdir -p "$PKG_PATH/data" "$PKG_PATH/data/backups"
chmod 700 "$PKG_PATH/data" "$PKG_PATH/data/backups" 2>/dev/null || true
chmod 755 "$PKG_PATH/cron-manager" "$PKG_PATH/CONTROL/start-stop.sh" 2>/dev/null || true
exit 0
