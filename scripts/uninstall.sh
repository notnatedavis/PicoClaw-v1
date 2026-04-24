# scripts/uninstall.sh

#!/usr/bin/env bash
# Completely removes the PicoClaw binary, configuration, and runtime data
# Prompts for confirmation before deletion

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "WARNING: This will delete the picoclaw binary, all logs, and the workspace."
read -p "Are you sure? (type 'yes' to confirm): " CONFIRM
if [ "$CONFIRM" != "yes" ]; then
    echo "Aborted."
    exit 1
fi

# Stop gateway first
bash scripts/stop.sh || true

# Remove binary and PID
rm -f picoclaw picoclaw.pid

# Remove logs
rm -rf logs/*.log

# Clear workspace (but keep .gitkeep)
find workspace/agent-sessions/ -mindepth 1 -not -name '.gitkeep' -exec rm -rf {} \;

echo "Uninstall complete. Configuration files are left intact; delete them manually if desired."