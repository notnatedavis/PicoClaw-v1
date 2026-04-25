#!/usr/bin/env bash
# scripts/setup.sh

# One‑command setup :
#   - creates directories
#   - copies .env.example → .env if missing
#   - downloads the PicoClaw binary
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

# 4. download the latest PicoClaw binary if not present
if [ ! -f picoclaw ]; then
    echo "    Downloading PicoClaw binary..."
    # detect OS and architecture
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        armv7l)  ARCH="armv7" ;;
        i686)    ARCH="386" ;;
        *)
            echo "    [WARN] Unknown architecture $ARCH. Attempting generic download"
            ARCH="amd64"
            ;;
    esac
    BIN_URL="https://github.com/sipeed/picoclaw/releases/latest/download/picoclaw-${OS}-${ARCH}"

    if command -v curl &> /dev/null; then
        curl -L "$BIN_URL" -o picoclaw
    else
        wget -O picoclaw "$BIN_URL"
    fi
    chmod +x picoclaw
    echo "    Binary downloaded and made executable"
else
    echo "    PicoClaw binary already present. Skipping download"
fi

# 5. ensure PICOCLAW_CONFIG is set for this session (no permanent shell rc changes)
export PICOCLAW_CONFIG="$REPO_ROOT/config/config.json"

echo "==> Setup complete"
echo "    Next: bash scripts/start.sh"