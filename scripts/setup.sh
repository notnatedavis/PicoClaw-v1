# scripts/setup.sh

#!/usr/bin/env bash
# One-command setup
#   downloads PicoClaw binary, sets up directories, and ensures 
#   environment is ready

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "==> Setting up PicoClaw environment..."

# 1. Ensure .env exists
if [ ! -f .env ]; then
    echo "    [ERROR] .env file missing. Please copy .env.example to .env and fill in your keys."
    exit 1
fi

# 2. Create required directories
mkdir -p logs workspace/agent-sessions/default

# 3. Download the latest PicoClaw binary if not present
if [ ! -f picoclaw ]; then
    echo "    Downloading PicoClaw binary..."
    # Detect OS and architecture
    OS="linux"
    ARCH="amd64"
    # Basic detection (you can expand for ARM, macOS)
    if [[ "$(uname -m)" == "aarch64" ]]; then ARCH="arm64"; fi
    BIN_URL="https://github.com/sipeed/picoclaw/releases/latest/download/picoclaw-${OS}-${ARCH}"
    
    if command -v curl &> /dev/null; then
        curl -L "$BIN_URL" -o picoclaw
    else
        wget -O picoclaw "$BIN_URL"
    fi
    chmod +x picoclaw
    echo "    Binary downloaded and made executable."
else
    echo "    PicoClaw binary already present. Skipping download."
fi

# 4. Set environment variable for config path (persistent)
export PICOCLAW_CONFIG="$REPO_ROOT/config/config.json"
if ! grep -q "PICOCLAW_CONFIG" ~/.bashrc 2>/dev/null; then
    echo "    Adding PICOCLAW_CONFIG to ~/.bashrc..."
    echo "export PICOCLAW_CONFIG=\"$PICOCLAW_CONFIG\"" >> ~/.bashrc
fi
# Also export for current session
export PICOCLAW_CONFIG="$REPO_ROOT/config/config.json"

echo "==> Setup complete."
echo "    Run 'source ~/.bashrc' or restart your shell to apply environment."
echo "    Next: bash scripts/start.sh"