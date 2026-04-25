#!/usr/bin/env bash
# scripts/stop.sh

# Gracefully stops the PicoClaw gateway process

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

if [ ! -f picoclaw.pid ]; then
    echo "No PID file found. Is PicoClaw running?"
    exit 1
fi

PID=$(cat picoclaw.pid)
if kill -0 "$PID" 2>/dev/null; then
    echo "Stopping PicoClaw (PID $PID)..."
    kill "$PID"
    sleep 1
    # Force kill if still alive
    if kill -0 "$PID" 2>/dev/null; then
        echo "Process did not stop, sending SIGKILL..."
        kill -9 "$PID"
    fi
    rm -f picoclaw.pid
    echo "PicoClaw stopped."
else
    echo "Process $PID is not running. Removing stale PID file."
    rm -f picoclaw.pid
fi