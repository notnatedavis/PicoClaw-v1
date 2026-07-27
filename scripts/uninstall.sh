#!/usr/bin/env bash
# scripts/uninstall.sh

# Cleans runtime logs only.
# Does NOT remove binaries or configuration – the binary is never downloaded
# by setup, so it stays as a local copy.

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "WARNING: This will delete all log files in logs/."
read -p "Are you sure? (type 'yes' to confirm): " CONFIRM
if [ "$CONFIRM" != "yes" ]; then
    echo "Aborted."
    exit 1
fi

# Stop gateway first (ignore errors if not running)
bash scripts/stop.sh || true

# Remove logs
rm -f logs/*.log
echo "All logs removed."
echo "Uninstall (logs only) complete."