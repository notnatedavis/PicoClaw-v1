#!/usr/bin/env bash
# scripts/setup.sh

# One‑command setup :
#   - creates directories
#   - copies .env.example → .env if missing
#   - auto‑detects OS/architecture and copies the matching binary from binaries/
#   - renames it to 'picoclaw' in the project root
#   - sets environment variables

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

# Helper to detect architecture (arm64 / x86 / unknown)
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "x86" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             echo "unknown" ;;
    esac
}

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

# 4. detect OS and architecture
OS=$(detect_os)
ARCH=$(detect_arch)

if [ "$OS" = "unknown" ] || [ "$ARCH" = "unknown" ]; then
    echo "    [ERROR] Unsupported OS/architecture: $(uname -s) / $(uname -m)"
    echo "            Only macOS (Darwin) and Windows (Git Bash / MSYS2) are supported."
    exit 1
fi

# Build the source binary filename
EXT=""
if [ "$OS" = "win" ]; then
    EXT=".exe"
fi
SOURCE_BINARY="binaries/picoclaw-binary-${OS}-${ARCH}${EXT}"

if [ ! -f "$SOURCE_BINARY" ]; then
    echo "    [ERROR] Binary not found: $SOURCE_BINARY"
    echo "            Please ensure the binaries are placed in the 'binaries/' directory."
    exit 1
fi

echo "    Detected OS: $OS, Architecture: $ARCH"
echo "    Copying $SOURCE_BINARY to ./picoclaw ..."

# Copy the binary to the root (overwrites any existing one)
cp "$SOURCE_BINARY" picoclaw
chmod +x picoclaw

# 5. ensure PICOCLAW_CONFIG is set for this session
export PICOCLAW_CONFIG="$REPO_ROOT/config/config.json"

echo "==> Setup complete"
echo "    Next: bash scripts/start.sh"