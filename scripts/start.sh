#!/usr/bin/env bash
# scripts/start.sh

# Starts the PicoClaw gateway in background, sourcing API keys and setting the config path.
# Automatically stops any stale/leftover processes before starting.
# Now includes architecture check and foreground validation to surface startup errors.

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

if [ ! -f picoclaw ]; then
    echo "[ERROR] PicoClaw binary not found. Run 'bash scripts/setup.sh' first"
    exit 1
fi

# load environment variables (API keys) from .env
if [ -f .env ]; then
    set -a; source .env; set +a
else
    echo "[ERROR] .env file missing. Cannot start without API keys"
    exit 1
fi

# set config path (environment may already have it)
export PICOCLAW_CONFIG="${PICOCLAW_CONFIG:-$REPO_ROOT/config/config.json}"

# --- Architecture / binary compatibility check ---
echo "==> Checking binary compatibility..."
BINARY_TYPE=$(file -b picoclaw 2>/dev/null || true)
CURRENT_ARCH=$(uname -m)
if [[ "$BINARY_TYPE" != *"Mach-O"* ]]; then
    echo "[ERROR] picoclaw binary is not a macOS executable."
    echo "       Detected type: $BINARY_TYPE"
    echo "       Please download the correct macOS binary from https://github.com/sipeed/picoclaw/releases"
    exit 1
fi
echo "    Binary appears to be a macOS executable."

# --- Dry-run foreground validation (captures panics, missing libraries, config errors) ---
echo "==> Verifying gateway can start (foreground trial for 3s)..."
set +e
timeout 3 ./picoclaw gateway > /tmp/picoclaw_trial.log 2>&1
TRIAL_EXIT=$?
set -e
if [ $TRIAL_EXIT -eq 124 ]; then
    echo "    Gateway started successfully (timeout reached – good)."
elif [ $TRIAL_EXIT -eq 0 ]; then
    echo "    Gateway exited cleanly after test (possibly daemonised or finished)."
else
    echo "[ERROR] Gateway failed to start (exit code $TRIAL_EXIT)."
    echo "-------- trial output --------"
    cat /tmp/picoclaw_trial.log
    rm -f /tmp/picoclaw_trial.log
    exit 1
fi
rm -f /tmp/picoclaw_trial.log

# --- Cleanup any stale/leftover processes ---
cleanup_stale() {
    # 1. Kill by PID file if it exists but process is dead or unresponsive
    if [ -f picoclaw.pid ]; then
        OLD_PID=$(cat picoclaw.pid)
        if kill -0 "$OLD_PID" 2>/dev/null; then
            echo "Found running PicoClaw instance (PID $OLD_PID). Stopping it first..."
            kill "$OLD_PID" 2>/dev/null || true
            sleep 1
            # Force kill if still alive
            if kill -0 "$OLD_PID" 2>/dev/null; then
                echo "Process did not stop gracefully, force killing..."
                kill -9 "$OLD_PID" 2>/dev/null || true
                sleep 0.5
            fi
            echo "Previous instance stopped."
        else
            echo "Found stale PID file (process $OLD_PID not running). Removing..."
        fi
        rm -f picoclaw.pid
    fi

    # 2. Kill any orphaned picoclaw processes that might be lingering
    ORPHANS=$(pgrep -f "picoclaw gateway" 2>/dev/null || true)
    if [ -n "$ORPHANS" ]; then
        echo "Found orphaned picoclaw processes: $ORPHANS"
        echo "Cleaning up orphans..."
        echo "$ORPHANS" | xargs kill 2>/dev/null || true
        sleep 1
        # Force kill any survivors
        SURVIVORS=$(pgrep -f "picoclaw gateway" 2>/dev/null || true)
        if [ -n "$SURVIVORS" ]; then
            echo "$SURVIVORS" | xargs kill -9 2>/dev/null || true
        fi
        echo "Orphans cleaned."
    fi
}

# Run cleanup before starting
cleanup_stale

# --- Start fresh ---
echo "Starting PicoClaw gateway in background..."
nohup ./picoclaw gateway > logs/picoclaw.log 2>&1 &
PID=$!
echo $PID > picoclaw.pid

# Verify it actually started
sleep 0.5
if kill -0 "$PID" 2>/dev/null; then
    echo "Started successfully with PID $PID. Logs: logs/picoclaw.log"
else
    echo "[ERROR] PicoClaw failed to start in background. Check logs/picoclaw.log for details."
    # Show the log tail to help immediate debugging
    echo "--- Last 20 lines of log ---"
    tail -n 20 logs/picoclaw.log || true
    echo "-----------------------------"
    rm -f picoclaw.pid
    exit 1
fi