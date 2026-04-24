# scripts/backup-agent-logs.sh

#!/usr/bin/env bash
# Creates a gzipped tarball of the logs/ directory with a timestamp

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="logs_backup_${TIMESTAMP}.tar.gz"

if [ ! -d logs ]; then
    echo "No logs directory found. Nothing to backup."
    exit 0
fi

tar -czf "$BACKUP_FILE" logs/
echo "Backup created: $BACKUP_FILE"