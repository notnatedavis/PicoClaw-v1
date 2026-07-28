#!/usr/bin/env bash
# scripts/debug-groq-env.sh

# Standalone diagnostic: confirms that GROQ_API_KEY is set in the shell,
# tests a direct Groq API call (like health-check), and if a PicoClaw
# gateway is running, inspects its environment for the key.
#
# Run this while the bot is active (or after a failed message) to see
# exactly where the key is being lost.

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "=== PicoClaw Groq API Key Debug ==="
echo ""

# 1. Check the key in the current shell (sourced from .env)
if [ -f .env ]; then
    set +e; set -a; source .env 2>/dev/null; set +a; set -e
    if [ -n "${GROQ_API_KEY:-}" ]; then
        echo "[OK] GROQ_API_KEY is set in shell: length=${#GROQ_API_KEY}, prefix='${GROQ_API_KEY:0:5}...'"
    else
        echo "[FAIL] GROQ_API_KEY is NOT set in .env or empty."
        exit 1
    fi
else
    echo "[FAIL] .env file not found."
    exit 1
fi

# 2. Test a direct Groq API call (same logic as health-check)
echo ""
echo "==> Testing direct Groq API call..."
HTTP_CODE=$(curl -s -w "%{http_code}" -X POST "https://api.groq.com/openai/v1/chat/completions" \
    -H "Authorization: Bearer $GROQ_API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"model":"llama-3.3-70b-versatile","messages":[{"role":"user","content":"say hi"}],"max_tokens":3}' \
    --max-time 10 \
    -o /tmp/debug_groq_test.json 2>/dev/null)
if [ "$HTTP_CODE" -eq 200 ]; then
    echo "[OK] Direct Groq call succeeded (HTTP 200). The key itself is valid."
else
    echo "[FAIL] Direct Groq call failed (HTTP $HTTP_CODE). Check .env key value."
    cat /tmp/debug_groq_test.json
    exit 1
fi
rm -f /tmp/debug_groq_test.json

# 3. Check the environment of a running gateway process
PID=""
if [ -f picoclaw.pid ]; then
    PID=$(cat picoclaw.pid)
fi

if [ -z "$PID" ] || ! kill -0 "$PID" 2>/dev/null; then
    echo ""
    echo "[INFO] No running PicoClaw gateway detected (or PID file missing)."
    echo "       Start the gateway first, then re-run this script."
    exit 0
fi

echo ""
echo "==> Inspecting environment of gateway process (PID $PID)..."

if [ -f /proc/$PID/environ ]; then
    # Extract and display the GROQ_API_KEY line (masked)
    KEY_LINE=$(tr '\0' '\n' < /proc/$PID/environ | grep '^GROQ_API_KEY=' || true)
    if [ -n "$KEY_LINE" ]; then
        # Show only prefix
        PREFIX=$(echo "$KEY_LINE" | sed 's/\(GROQ_API_KEY=......\).*/\1.../')
        echo "[OK] GROQ_API_KEY found in process environment: $PREFIX"
        echo "     -> The gateway can see the key. If 401 persists, the binary may expect a different variable name or config setting."
    else
        echo "[FAIL] GROQ_API_KEY is NOT in the gateway's environment."
        echo "     This explains the 'Invalid API Key' error."
        echo ""
        echo "     Quick fix: stop the gateway, manually export the key, and restart:"
        echo "         bash scripts/stop.sh"
        echo "         export GROQ_API_KEY='your-key-here'"
        echo "         bash scripts/start.sh"
        echo ""
        echo "     Permanent fix: ensure start.sh exports GROQ_API_KEY (already done in the updated script)."
    fi
else
    echo "[WARN] Cannot read /proc/$PID/environ (not available on this platform)."
    echo "       To manually test, run the gateway in the foreground with the key exported:"
    echo "           export GROQ_API_KEY='your-key-here'"
    echo "           ./picoclaw gateway"
    echo "       Then send a message on Telegram and watch the terminal output."
fi

echo ""
echo "=== Debug complete ==="