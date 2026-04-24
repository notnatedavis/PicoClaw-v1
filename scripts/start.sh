# scripts/start.sh

#!/usr/bin/env bash
# Starts the PicoClaw gateway in background, sourcing API keys and setting the config path.

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

# check if already running
if [ -f picoclaw.pid ] && kill -0 "$(cat picoclaw.pid)" 2>/dev/null; then
    echo "PicoClaw is already running with PID $(cat picoclaw.pid)"
    exit 0
fi

echo "Starting PicoClaw gateway..."
nohup ./picoclaw gateway > logs/picoclaw.log 2>&1 &
PID=$!
echo $PID > picoclaw.pid
echo "Started with PID $PID. Logs: logs/picoclaw.log"