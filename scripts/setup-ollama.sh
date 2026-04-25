#!/usr/bin/env bash
# scripts/setup-ollama.sh

# Installs Ollama and pulls a lightweight free model for local, offline use.
# Requires curl and systemd (Linux)

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "==> Installing Ollama (local AI runtime)..."
if command -v ollama &> /dev/null; then
    echo "    Ollama already installed."
else
    curl -fsSL https://ollama.com/install.sh | sh
fi

echo "==> Pulling a small, free model (qwen2.5:1.5b) – about 1 GB..."
ollama pull qwen2.5:1.5b

echo "==> Ollama ready. To use it with PicoClaw, add this block to config.json:"
echo '    "model_list": {'
echo '      "local-qwen": {'
echo '        "provider": "ollama",'
echo '        "api_base": "http://localhost:11434",'
echo '        "model": "qwen2.5:1.5b"'
echo '      }'
echo '    }'