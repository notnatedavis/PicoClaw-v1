#!/usr/bin/env bash
# scripts/clean.sh

# Removes log files + empties workspace (keeping directory structure)
# Does NOT remove (config/ or picoclaw)

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "Cleaning logs and workspace..."

# Clear logs
rm -f logs/*.log

# Clear workspace but keep .gitkeep
find workspace/agent-sessions/ -mindepth 1 -maxdepth 1 -not -name '.gitkeep' -exec rm -rf {} \;

# Remove PID file
rm -f picoclaw.pid

echo "Cleanup done."