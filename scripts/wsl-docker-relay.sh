#!/usr/bin/env bash
# Expose the WSL Docker daemon's unix socket on 127.0.0.1:2375 (plain TCP) so
# the Windows `docker` CLI — and therefore BAT's Docker wizard, which runs
# `docker` on Windows — can talk to the daemon that lives inside WSL (D113).
# Tested on T0193 with WSL networkingMode=Mirrored: Windows 127.0.0.1:2375
# reaches this listener directly.
#
# No sudo, no dockerd config change: it runs as your WSL user, who is in the
# `docker` group. SECURITY: anything on this machine that can open
# 127.0.0.1:2375 gets root-equivalent access to WSL (same trust as the
# docker group / docker.sock). Loopback only; never bind 0.0.0.0.
#
#   scripts/wsl-docker-relay.sh start | stop | status
# Then on Windows (once):
#   docker context create wsl --docker host=tcp://127.0.0.1:2375
#   docker context use wsl          # or set DOCKER_HOST=tcp://127.0.0.1:2375 before launching BAT
set -euo pipefail

PORT=${PORT:-2375}
PIDFILE=${PIDFILE:-$HOME/.wsl-docker-relay.pid}
LOG=${LOG:-$HOME/.wsl-docker-relay.log}

running() { [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; }

case "${1:-status}" in
  start)
    command -v socat >/dev/null || { echo "socat missing: sudo apt-get install -y socat" >&2; exit 1; }
    if running; then echo "already running (pid $(cat "$PIDFILE"))"; exit 0; fi
    setsid nohup socat "TCP-LISTEN:${PORT},bind=127.0.0.1,reuseaddr,fork" UNIX-CONNECT:/var/run/docker.sock \
      > "$LOG" 2>&1 < /dev/null &
    echo $! > "$PIDFILE"
    sleep 0.5
    docker -H "tcp://127.0.0.1:${PORT}" version --format 'relay ok: server={{.Server.Version}}'
    ;;
  stop)
    if running; then kill "$(cat "$PIDFILE")" && rm -f "$PIDFILE" && echo stopped; else rm -f "$PIDFILE"; echo "not running"; fi
    ;;
  status)
    if running; then echo "running (pid $(cat "$PIDFILE")) on 127.0.0.1:${PORT}"; else echo "not running"; exit 1; fi
    ;;
  *) echo "usage: $0 {start|stop|status}" >&2; exit 2 ;;
esac
