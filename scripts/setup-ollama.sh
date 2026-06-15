#!/usr/bin/env bash
# scripts/setup-ollama.sh

# Installs Ollama

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "==> Installing Ollama ..."
if command -v ollama &> /dev/null; then
    echo "    Ollama already installed."
else
    curl -fsSL https://ollama.com/install.sh | sh
fi