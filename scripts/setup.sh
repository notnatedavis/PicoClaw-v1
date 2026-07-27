#!/usr/bin/env bash
# scripts/setup.sh

# One‑command setup :
#   - creates directories
#   - copies .env.example → .env if missing
#   - auto‑detects OS and copies the matching platform binary to 'picoclaw'
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

# 4. detect OS and pick the correct platform binary
detected_os="unknown"
case "$(uname -s)" in
    Darwin)  detected_os="mac" ;;
    MINGW*|MSYS*|CYGWIN*)  detected_os="win" ;;
esac

platform_binary=""
if [ "$detected_os" = "mac" ]; then
    platform_binary="picoclaw-binary-mac"
elif [ "$detected_os" = "win" ]; then
    platform_binary="picoclaw-binary-win"
else
    echo "    [ERROR] Unsupported operating system: $(uname -s)."
    echo "            Only macOS (Darwin) and Windows (Git Bash / MSYS2) are supported."
    exit 1
fi

# If 'picoclaw' already exists (from a previous setup) keep it
# Otherwise, copy the platform‑specific binary
if [ ! -f picoclaw ]; then
    if [ -f "$platform_binary" ]; then
        echo "    Creating 'picoclaw' from $platform_binary (detected OS: $detected_os)..."
        cp "$platform_binary" picoclaw
        chmod +x picoclaw
        echo "    Platform binary copied and made executable."
    else
        echo "    [ERROR] '$platform_binary' not found."
        echo "            Please download the correct binary from https://github.com/sipeed/picoclaw/releases"
        exit 1
    fi
else
    # binary already present – ensure it’s executable
    if [ ! -x picoclaw ]; then
        echo "    picoclaw found but not executable. Making it executable..."
        chmod +x picoclaw
    fi
    echo "    picoclaw binary already present."
fi

# 5. ensure PICOCLAW_CONFIG is set for this session
export PICOCLAW_CONFIG="$REPO_ROOT/config/config.json"

echo "==> Setup complete"
echo "    Next: bash scripts/start.sh"