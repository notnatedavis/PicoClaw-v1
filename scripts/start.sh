#!/usr/bin/env bash
# scripts/start.sh

# Starts the PicoClaw gateway in background, sourcing API keys and setting the config path.
# Automatically stops any stale/leftover processes before starting.
# Includes architecture check, foreground validation, and port conflict resolution.
# Ensures TELEGRAM_TOKEN is set for the binary.
# Also writes the Telegram bot token and Groq API key to ~/.picoclaw/.security.yml
# as required by the gateway (v0.3.1 reads secrets from this file, not from config.json).

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

if [ ! -f picoclaw ]; then
    echo "[ERROR] PicoClaw binary not found. Run 'bash scripts/setup.sh' first"
    exit 1
fi

# Helper to detect OS (mac / win / unknown)
detect_os() {
    case "$(uname -s)" in
        Darwin)  echo "mac" ;;
        MINGW*|MSYS*|CYGWIN*)  echo "win" ;;
        *)       echo "unknown" ;;
    esac
}

# Helper to find and kill process using a given port (cross‑platform)
kill_process_on_port() {
    local port="$1"
    local pid=""
    local os
    os=$(detect_os)

    case "$os" in
        win)
            # Use || true to prevent script exit when no process is found
            pid=$(netstat -ano 2>/dev/null | grep ":$port " | grep LISTENING | awk '{print $NF}' | head -n1 || true)
            if [ -n "$pid" ]; then
                echo "    Found process $pid using port $port. Killing it..."
                taskkill //F //PID "$pid" >/dev/null 2>&1 || true
                sleep 1
            fi
            ;;
        mac|linux|*)
            pid=$(lsof -t -i :"$port" 2>/dev/null | head -n1 || true)
            if [ -n "$pid" ]; then
                echo "    Found process $pid using port $port. Killing it..."
                kill -9 "$pid" >/dev/null 2>&1 || true
                sleep 1
            fi
            ;;
    esac
}

# load environment variables (API keys) from .env
if [ -f .env ]; then
    set -a; source .env; set +a
else
    echo "[ERROR] .env file missing. Cannot start without API keys"
    exit 1
fi

# --- Explicitly export GROQ_API_KEY so it is guaranteed visible ---
if [ -n "${GROQ_API_KEY:-}" ]; then
    export GROQ_API_KEY="${GROQ_API_KEY}"
    echo "==> GROQ_API_KEY present: length=${#GROQ_API_KEY}, starts with '${GROQ_API_KEY:0:5}...'"
else
    echo "[ERROR] GROQ_API_KEY is empty or not set in .env. The bot cannot use Groq."
    exit 1
fi

# Ensure TELEGRAM_TOKEN is set (the binary uses this variable)
if [ -z "${TELEGRAM_TOKEN:-}" ] && [ -n "${TELEGRAM_BOT_TOKEN:-}" ]; then
    export TELEGRAM_TOKEN="$TELEGRAM_BOT_TOKEN"
    echo "    TELEGRAM_TOKEN set from TELEGRAM_BOT_TOKEN"
fi

# ---- Write ALL secrets to ~/.picoclaw/.security.yml ----
# The gateway expects credentials in this file, not in config.json.
# We overwrite the file each time to keep it in sync with .env.
SECURITY_DIR="$HOME/.picoclaw"
SECURITY_FILE="$SECURITY_DIR/.security.yml"
mkdir -p "$SECURITY_DIR"

# Build the file with both Telegram and Groq credentials
{
    echo "channels:"
    echo "  telegram:"
    if [ -n "${TELEGRAM_BOT_TOKEN:-}" ]; then
        echo "    token: \"${TELEGRAM_BOT_TOKEN}\""
    else
        echo "    token: \"\""
    fi
    echo ""
    echo "providers:"
    echo "  groq:"
    echo "    api_key: \"${GROQ_API_KEY}\""
    # add other providers here if needed
} > "$SECURITY_FILE"

echo "    Wrote Telegram token and Groq API key to $SECURITY_FILE"

# Also set the environment variable that the gateway might read (fallback)
if [ -n "${TELEGRAM_BOT_TOKEN:-}" ]; then
    export PICOCLAW_CHANNELS_TELEGRAM_TOKEN="$TELEGRAM_BOT_TOKEN"
fi

# --- Quick Telegram token validation (early warning) ---
if [ -n "${TELEGRAM_BOT_TOKEN:-}" ]; then
    echo "==> Testing Telegram bot token..."
    HTTP_STATUS=$(curl -s -w "%{http_code}" \
        "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getMe" \
        --max-time 5 \
        -o /tmp/telegram_start_test.json 2>/dev/null)
    if [ "$HTTP_STATUS" -eq 200 ] && grep -q '"ok":true' /tmp/telegram_start_test.json; then
        BOT_USER=$(jq -r '.result.username' /tmp/telegram_start_test.json 2>/dev/null || echo "unknown")
        echo "    Telegram token valid (bot @$BOT_USER)"
    else
        echo "[WARN] Telegram token validation failed (HTTP $HTTP_STATUS)."
        echo "       The gateway may still start, but the bot might not respond."
        echo "       Check your token and network connectivity."
    fi
    rm -f /tmp/telegram_start_test.json
else
    echo "[WARN] TELEGRAM_BOT_TOKEN is not set in .env. Bot will not work."
fi

# set config path to the original config (no runtime injection needed)
export PICOCLAW_CONFIG="$REPO_ROOT/config/config.json"
echo "    PICOCLAW_CONFIG set to $PICOCLAW_CONFIG"

# --- Architecture / binary compatibility check (platform‑aware) ---
echo "==> Checking binary compatibility..."
OS=$(detect_os)
BINARY_TYPE=$(file -b picoclaw 2>/dev/null || true)

case "$OS" in
    mac)
        if [[ "$BINARY_TYPE" == *"Mach-O"* ]]; then
            echo "    Binary is a macOS executable."
        else
            echo "[ERROR] picoclaw binary is not a macOS executable."
            echo "       Detected type: $BINARY_TYPE"
            echo "       Please run 'bash scripts/setup.sh' to copy the correct binary."
            exit 1
        fi
        ;;
    win)
        if [[ "$BINARY_TYPE" == *"PE32"* ]]; then
            echo "    Binary is a Windows executable."
        else
            echo "[ERROR] picoclaw binary is not a Windows executable."
            echo "       Detected type: $BINARY_TYPE"
            echo "       Please run 'bash scripts/setup.sh' to copy the correct binary."
            exit 1
        fi
        ;;
    *)
        echo "[WARN] Unknown OS, skipping binary type check."
        ;;
esac

# --- Ensure the gateway port is free (18790 by default) ---
echo "==> Checking port availability (18790)..."
kill_process_on_port 18790

# --- Dry-run foreground validation (captures panics, missing libraries, config errors) ---
echo "==> Verifying gateway can start (foreground trial for 3s)..."
if command -v timeout >/dev/null 2>&1; then
    set +e
    timeout 3 ./picoclaw gateway > /tmp/picoclaw_trial.log 2>&1
    TRIAL_EXIT=$?
    set -e
else
    echo "    'timeout' command not found; skipping foreground test (gateway will start directly)."
    TRIAL_EXIT=0
fi

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
    if [ -f picoclaw.pid ]; then
        OLD_PID=$(cat picoclaw.pid)
        if kill -0 "$OLD_PID" 2>/dev/null; then
            echo "Found running PicoClaw instance (PID $OLD_PID). Stopping it first..."
            kill "$OLD_PID" 2>/dev/null || true
            sleep 1
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

    if command -v pgrep >/dev/null 2>&1; then
        ORPHANS=$(pgrep -f "picoclaw gateway" 2>/dev/null || true)
        if [ -n "$ORPHANS" ]; then
            echo "Found orphaned picoclaw processes: $ORPHANS"
            echo "Cleaning up orphans..."
            echo "$ORPHANS" | xargs kill 2>/dev/null || true
            sleep 1
            SURVIVORS=$(pgrep -f "picoclaw gateway" 2>/dev/null || true)
            if [ -n "$SURVIVORS" ]; then
                echo "$SURVIVORS" | xargs kill -9 2>/dev/null || true
            fi
            echo "Orphans cleaned."
        fi
    else
        echo "    pgrep not available; skipping orphan cleanup (port kill will handle)."
    fi
}

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
    echo "--- Last 20 lines of log ---"
    tail -n 20 logs/picoclaw.log || true
    echo "-----------------------------"
    rm -f picoclaw.pid
    exit 1
fi

# --- Optional env check ---
if [ -f /proc/$PID/environ ]; then
    echo "==> Checking GROQ_API_KEY in gateway process environment (PID $PID)..."
    if tr '\0' '\n' < /proc/$PID/environ | grep -q '^GROQ_API_KEY='; then
        echo "    [OK] GROQ_API_KEY found in process environment."
    else
        echo "    [WARN] GROQ_API_KEY NOT present in process environment."
    fi
else
    echo "    (Cannot check process environment – /proc/$PID/environ not available on this system)"
fi

echo ""
echo "The Groq API key is now stored in $SECURITY_FILE."
echo "If the bot still returns 'Invalid API Key' on free-text messages, run:"
echo "    cat $SECURITY_FILE"
echo "to verify the key is correctly written."