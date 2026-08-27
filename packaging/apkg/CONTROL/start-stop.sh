#!/bin/sh
set -u

PKG_PATH="/usr/local/AppCentral/CronManager"
BIN="$PKG_PATH/cron-manager"
DATA="$PKG_PATH/data"
LOG="$PKG_PATH/cron-manager.log"
PIDFILE="$PKG_PATH/cron-manager.pid"

export PATH="/usr/local/bin:/usr/builtin/bin:/usr/bin:/bin:$PATH"
export ACM_DATA="$DATA"
export ACM_RUNNER="$BIN"
export ACM_LISTEN="0.0.0.0:18367"

is_running() {
    [ -f "$PIDFILE" ] || return 1
    pid="$(cat "$PIDFILE" 2>/dev/null || true)"
    [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

start() {
    if is_running; then
        exit 0
    fi
    mkdir -p "$DATA" "$DATA/backups"
    chmod 700 "$DATA" "$DATA/backups" 2>/dev/null || true
    chmod 755 "$BIN" 2>/dev/null || true
    cd "$PKG_PATH" || exit 1
    nohup "$BIN" >>"$LOG" 2>&1 &
    echo $! > "$PIDFILE"
}

stop() {
    if is_running; then
        pid="$(cat "$PIDFILE")"
        kill "$pid" 2>/dev/null || true
        for _ in 1 2 3 4 5; do
            kill -0 "$pid" 2>/dev/null || break
            sleep 1
        done
        kill -9 "$pid" 2>/dev/null || true
    fi
    rm -f "$PIDFILE"
}

case "${1:-}" in
    start) start ;;
    stop) stop ;;
    restart) stop; sleep 1; start ;;
    status) if is_running; then echo "running"; else echo "stopped"; exit 1; fi ;;
    *) echo "usage: $0 {start|stop|restart|status}"; exit 2 ;;
esac
