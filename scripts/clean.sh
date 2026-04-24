# scripts/clean.sh

#!/usr/bin/env bash
# Removes log files and empties workspace (keeps directory structure)
# Does NOT remove configuration or the binary

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