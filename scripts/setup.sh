#!/usr/bin/env bash
# scripts/setup.sh

# One‑command setup :
#   - creates directories
#   - copies .env.example → .env if missing
#   - VERIFIES local picoclaw binary (does NOT download)
#   - sets environment variables

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "==> Setting up PicoClaw environment..."

# 1. prepare .env file
if [ ! -f .env ]; then
    if [ -f .env.example ]; then
        echo "    Creating .env from .env.example. Please edit it with your keys"
        cp .env.example .env
        echo "    Run: nano .env   when ready"
    else
        echo "    [ERROR] No .env.example found. Cannot continue"
        exit 1
    fi
fi

# source .env robustly (ignore errors, export everything)
set +e; set -a; source .env 2>/dev/null; set +a; set -e

# 2. create required directories
mkdir -p logs workspace/agent-sessions/default

# 3. remove stale PID if present (should not be running during setup)
rm -f picoclaw.pid

# 4. verify local picoclaw binary (no download)
if [ ! -f picoclaw ]; then
    echo "  [ERROR] picoclaw binary not found."
    exit 1
fi

if [ ! -x picoclaw ]; then
    echo "    picoclaw found but not executable. Making it executable..."
    chmod +x picoclaw
fi

echo "    Local picoclaw binary verified."

# 5. ensure PICOCLAW_CONFIG is set for this session
export PICOCLAW_CONFIG="$REPO_ROOT/config/config.json"

echo "==> Setup complete"
echo "    Next: bash scripts/start.sh"