# scripts/health-check.sh

#!/usr/bin/env bash
# Quick checks:
# - Binary exists
# - .env loaded
# - Groq API reachable
# - Telegram token set

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

ERRORS=0

# 1. Binary
if [ ! -f picoclaw ]; then
    echo "[FAIL] picoclaw binary not found."
    ERRORS=$((ERRORS+1))
else
    echo "[ OK ] picoclaw binary present."
fi

# 2. .env
if [ -f .env ]; then
    set -a; source .env; set +a
    echo "[ OK ] .env loaded."
else
    echo "[FAIL] .env missing."
    ERRORS=$((ERRORS+1))
fi

# 3. Groq API (simple connectivity test)
if [ -n "${GROQ_API_KEY:-}" ]; then
    if curl -s -o /dev/null -w "%{http_code}" \
       -H "Authorization: Bearer $GROQ_API_KEY" \
       "https://api.groq.com/openai/v1/models" | grep -q 200; then
        echo "[ OK ] Groq API reachable."
    else
        echo "[FAIL] Groq API unreachable or invalid key."
        ERRORS=$((ERRORS+1))
    fi
else
    echo "[FAIL] GROQ_API_KEY not set."
    ERRORS=$((ERRORS+1))
fi

# 4. Telegram token
if [ -n "${TELEGRAM_BOT_TOKEN:-}" ]; then
    echo "[ OK ] Telegram bot token is set."
else
    echo "[FAIL] TELEGRAM_BOT_TOKEN not set."
    ERRORS=$((ERRORS+1))
fi

if [ $ERRORS -eq 0 ]; then
    echo "All checks passed."
else
    echo "$ERRORS check(s) failed."
fi