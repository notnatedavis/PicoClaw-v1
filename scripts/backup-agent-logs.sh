#!/usr/bin/env bash
# scripts/backup-agent-logs.sh

# creates gzipped tarball of logs/ directory with timestamp

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