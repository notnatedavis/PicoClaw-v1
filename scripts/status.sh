#!/usr/bin/env bash
# scripts/status.sh

# Checks whether the PicoClaw gateway is running

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

if [ -f picoclaw.pid ] && kill -0 "$(cat picoclaw.pid)" 2>/dev/null; then
    echo "PicoClaw is running (PID $(cat picoclaw.pid))."
else
    echo "PicoClaw is stopped."
fi