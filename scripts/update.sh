# scripts/update.sh

#!/usr/bin/env bash
# Stops running gateway, replaces the binary with the latest release,
#    and (optionally) restarts

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "==> Updating PicoClaw binary..."
bash scripts/stop.sh || true   # stop if running, ignore errors if not

# Remove old binary
rm -f picoclaw

# Re-run the download part of setup
bash scripts/setup.sh

echo "==> Update complete"

# (validate this is correct here)
read -p "Restart the gateway now? (y/N): " RESTART
if [[ "$RESTART" =~ ^[Yy]$ ]]; then
    bash scripts/start.sh
else
    echo "Update complete. Start with: bash scripts/start.sh"
fi