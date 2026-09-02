#!/usr/bin/env bash
# scripts/stop.sh

# Gracefully stops the PicoClaw gateway process.
# Also kills any rogue process occupying port 18790.
# 2026‑07‑28: Removes the temporary runtime config created by start.sh.
# 2026‑08‑02: Clears workspace sessions and memory directories (excluding .gitkeep).

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Helper to detect OS (mac / win / unknown)
detect_os() {
    case "$(uname -s)" in
        Darwin)  echo "mac" ;;
        MINGW*|MSYS*|CYGWIN*)  echo "win" ;;
        *)       echo "unknown" ;;
    esac
}

# Kill process(es) using a given port (cross‑platform)
kill_process_on_port() {
    local port="$1"
    local os
    os=$(detect_os)
    local pids=""

    case "$os" in
        win)
            pids=$(netstat -ano 2>/dev/null | grep ":$port " | grep LISTENING | awk '{print $NF}' | sort -u || true)
            ;;
        mac|linux|*)
            pids=$(lsof -t -i :"$port" 2>/dev/null || true)
            ;;
    esac

    if [ -n "$pids" ]; then
        echo "Found process(es) using port $port: $pids"
        for pid in $pids; do
            echo "Killing PID $pid..."
            case "$os" in
                win)
                    taskkill //F //PID "$pid" >/dev/null 2>&1 || true
                    ;;
                *)
                    kill -9 "$pid" >/dev/null 2>&1 || true
                    ;;
            esac
        done
        sleep 0.5
        # Verify it's gone
        case "$os" in
            win)
                remaining=$(netstat -ano 2>/dev/null | grep ":$port " | grep LISTENING | awk '{print $NF}' | sort -u || true)
                ;;
            *)
                remaining=$(lsof -t -i :"$port" 2>/dev/null || true)
                ;;
        esac
        if [ -n "$remaining" ]; then
            echo "WARNING: Some processes still hold port $port: $remaining"
        else
            echo "Port $port is now free."
        fi
    else
        echo "No process found using port $port."
    fi
}

# 1. Kill via PID file (if exists and running)
if [ -f picoclaw.pid ]; then
    PID=$(cat picoclaw.pid)
    if kill -0 "$PID" 2>/dev/null; then
        echo "Stopping PicoClaw (PID $PID)..."
        kill "$PID" 2>/dev/null || true
        sleep 1
        if kill -0 "$PID" 2>/dev/null; then
            echo "Process did not stop, sending SIGKILL..."
            kill -9 "$PID" 2>/dev/null || true
        fi
        rm -f picoclaw.pid
        echo "PicoClaw stopped."
    else
        echo "Process $PID is not running. Removing stale PID file."
        rm -f picoclaw.pid
    fi
else
    echo "No PID file found."
fi

# 2. Force‑kill any remaining process on the gateway port (18790)
echo "Checking for rogue processes on port 18790..."
kill_process_on_port 18790

# 3. Remove the runtime config (with embedded API key) for security
RUNTIME_CONFIG="$REPO_ROOT/config/runtime-config.json"
if [ -f "$RUNTIME_CONFIG" ]; then
    rm -f "$RUNTIME_CONFIG"
    echo "Removed temporary runtime config: $RUNTIME_CONFIG"
fi

# 4. Clear workspace sessions and memory directories (preserving .gitkeep files)
echo "Clearing workspace sessions and memory..."
if [ -d workspace/sessions ]; then
    find workspace/sessions/ -mindepth 1 -maxdepth 1 -not -name '.gitkeep' -exec rm -rf {} \;
fi
if [ -d workspace/memory ]; then
    find workspace/memory/ -mindepth 1 -maxdepth 1 -not -name '.gitkeep' -exec rm -rf {} \;
fi

echo "Stop complete."