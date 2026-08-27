#!/bin/sh
# Convert managed Cron Manager blocks back into plain cron entries before the
# package disappears. This keeps scheduled commands independent of the app.
set -u
PKG_PATH="/usr/local/AppCentral/CronManager"
BIN="$PKG_PATH/cron-manager"
DATA="$PKG_PATH/data"
if [ -x "$BIN" ]; then
    ACM_DATA="$DATA" "$BIN" unmanage-all || {
        echo "Cron Manager: could not convert managed jobs back to plain cron entries." >&2
        echo "Uninstall aborted to avoid leaving broken scheduled jobs." >&2
        exit 1
    }
fi
exit 0
