#!/usr/bin/env bash
# scripts/health-check.sh

# Health Checks:
# - binary presence and type (platform‑aware)
# - .env presence,
# - API reachability (models list fetched once, reused for fallback)
# - binary compatibility
# - Groq API prompt response (actual LLM completion) – uses model from config.
#   If the configured model is decommissioned, a suitable fallback is auto‑detected
#   from the models list so the check never breaks due to a single deprecation.
# - Telegram bot token validity (getMe)
# - Optional: send a test message via Telegram if TEST_TELEGRAM_CHAT_ID is set
# - Port availability (18790) – if in use, automatically kills the rogue process.

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

ERRORS=0

# Helper to detect OS (mac / win / unknown)
detect_os() {
    case "$(uname -s)" in
        Darwin)  echo "mac" ;;
        MINGW*|MSYS*|CYGWIN*)  echo "win" ;;
        *)       echo "unknown" ;;
    esac
}

# Kill process(es) using a given port (cross‑platform)
kill_process_on_port() {
    local port="$1"
    local os
    os=$(detect_os)
    local pids=""

    case "$os" in
        win)
            pids=$(netstat -ano 2>/dev/null | grep ":$port " | grep LISTENING | awk '{print $NF}' | sort -u || true)
            ;;
        mac|linux|*)
            pids=$(lsof -t -i :"$port" 2>/dev/null || true)
            ;;
    esac

    if [ -n "$pids" ]; then
        echo "    Killing process(es) using port $port: $pids"
        for pid in $pids; do
            case "$os" in
                win)
                    taskkill //F //PID "$pid" >/dev/null 2>&1 || true
                    ;;
                *)
                    kill -9 "$pid" >/dev/null 2>&1 || true
                    ;;
            esac
        done
        sleep 0.5
        # Re‑check
        case "$os" in
            win)
                remaining=$(netstat -ano 2>/dev/null | grep ":$port " | grep LISTENING | awk '{print $NF}' | sort -u || true)
                ;;
            *)
                remaining=$(lsof -t -i :"$port" 2>/dev/null || true)
                ;;
        esac
        if [ -n "$remaining" ]; then
            echo "    WARNING: Some processes still hold port $port: $remaining"
        else
            echo "    Port $port is now free."
        fi
    else
        echo "    No process found using port $port."
    fi
}

# 1. binary presence and type check
if [ ! -f picoclaw ]; then
    echo "[FAIL] picoclaw binary not found"
    ERRORS=$((ERRORS+1))
else
    echo "[ OK ] picoclaw binary present"
    OS=$(detect_os)
    BINARY_TYPE=$(file -b picoclaw 2>/dev/null || echo "unknown")
    case "$OS" in
        mac)
            if [[ "$BINARY_TYPE" == *"Mach-O"* ]]; then
                echo "[ OK ] Binary is a macOS executable"
            else
                echo "[FAIL] Binary does not appear to be a macOS executable (type: $BINARY_TYPE)"
                ERRORS=$((ERRORS+1))
            fi
            ;;
        win)
            if [[ "$BINARY_TYPE" == *"PE32"* ]]; then
                echo "[ OK ] Binary is a Windows executable"
            else
                echo "[FAIL] Binary does not appear to be a Windows executable (type: $BINARY_TYPE)"
                ERRORS=$((ERRORS+1))
            fi
            ;;
        *)
            echo "[WARN] Unknown OS, skipping binary type check"
            ;;
    esac
fi

# 2. .env availability and loading
if [ -f .env ]; then
    set +e; set -a; source .env 2>/dev/null; set +a; set -e
    echo "[ OK ] .env loaded"
else
    echo "[FAIL] .env missing"
    ERRORS=$((ERRORS+1))
fi

# 3. Groq API key presence (needed for further checks)
if [ -z "${GROQ_API_KEY:-}" ]; then
    echo "[FAIL] GROQ_API_KEY not set"
    ERRORS=$((ERRORS+1))
fi

# 4. Telegram token and connectivity test
if [ -n "${TELEGRAM_BOT_TOKEN:-}" ]; then
    echo "[ OK ] Telegram bot token is set"
    echo "==> Testing Telegram Bot API (getMe) ..."
    HTTP_STATUS=$(curl -s -w "%{http_code}" \
        "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getMe" \
        --max-time 10 \
        -o /tmp/telegram_response.json 2>/dev/null)
    if [ "$HTTP_STATUS" -eq 200 ]; then
        if grep -q '"ok":true' /tmp/telegram_response.json; then
            BOT_USERNAME=$(jq -r '.result.username' /tmp/telegram_response.json 2>/dev/null || echo "unknown")
            echo "[ OK ] Telegram bot token is valid (bot @$BOT_USERNAME)"
        else
            echo "[FAIL] Telegram API returned ok:false. Check your token."
            ERRORS=$((ERRORS+1))
        fi
    else
        echo "[FAIL] Telegram API unreachable or invalid token (HTTP $HTTP_STATUS)"
        ERRORS=$((ERRORS+1))
    fi
    rm -f /tmp/telegram_response.json

    # Optional: test sending a message if a test chat ID is provided
    if [ -n "${TEST_TELEGRAM_CHAT_ID:-}" ]; then
        echo "==> Sending test message to Telegram chat $TEST_TELEGRAM_CHAT_ID ..."
        SEND_RESPONSE=$(mktemp)
        HTTP_SEND=$(curl -s -w "%{http_code}" -X POST \
            "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
            -d "chat_id=$TEST_TELEGRAM_CHAT_ID&text=🧪 Health-check test from PicoClaw" \
            --max-time 10 \
            -o "$SEND_RESPONSE" 2>/dev/null)
        if [ "$HTTP_SEND" -eq 200 ] && grep -q '"ok":true' "$SEND_RESPONSE"; then
            echo "[ OK ] Test message sent successfully."
        else
            echo "[WARN] Test message failed (HTTP $HTTP_SEND). Check chat ID and that the bot has started a chat with that user."
        fi
        rm -f "$SEND_RESPONSE"
    fi
else
    echo "[FAIL] TELEGRAM_BOT_TOKEN not set"
    ERRORS=$((ERRORS+1))
fi

# 5. Port availability check (gateway uses 18790) – automatically kill rogue processes
echo "==> Checking port 18790 availability..."
OS=$(detect_os)
PID=""
case "$OS" in
    win)
        # Use || true to prevent script exit if netstat/grep fail
        PID=$(netstat -ano 2>/dev/null | grep ":18790 " | grep LISTENING | awk '{print $NF}' | head -n1 || true)
        ;;
    mac|linux)
        PID=$(lsof -t -i :18790 2>/dev/null | head -n1 || true)
        ;;
    *)
        PID=""
        ;;
esac

if [ -n "$PID" ]; then
    echo "[WARN] Port 18790 is already in use by process $PID. Killing it now..."
    kill_process_on_port 18790
else
    echo "[ OK ] Port 18790 is free."
fi

# Only proceed with Groq API checks if we have a key
if [ -n "${GROQ_API_KEY:-}" ]; then
    # Fetch the models list once – we’ll reuse it for reachability + fallback.
    MODELS_FILE=$(mktemp)
    HTTP_STATUS=$(curl -s -w "%{http_code}" \
        -H "Authorization: Bearer $GROQ_API_KEY" \
        "https://api.groq.com/openai/v1/models" \
        --max-time 10 \
        -o "$MODELS_FILE" 2>/dev/null)

    if [ "$HTTP_STATUS" -eq 200 ]; then
        echo "[ OK ] Groq API reachable"
    else
        echo "[FAIL] Groq API unreachable or invalid key (HTTP $HTTP_STATUS)"
        ERRORS=$((ERRORS+1))
    fi

    # Read configured model from config.json
    CONFIG_FILE="$REPO_ROOT/config/config.json"
    if [ -f "$CONFIG_FILE" ] && command -v jq >/dev/null 2>&1; then
        MODEL=$(jq -r '.agents.defaults.model_name // .model_list[0].model // "llama-3.3-70b-versatile"' "$CONFIG_FILE")
    else
        MODEL="llama-3.3-70b-versatile"   # last‑resort fallback
        echo "[WARN] Could not read config or jq missing; using fallback model: $MODEL"
    fi

    # Helper: run a simple completion test and return 0 on success
    test_model() {
        local model="$1"
        local outfile
        outfile=$(mktemp)
        local http_code
        http_code=$(curl -s -w "%{http_code}" -X POST "https://api.groq.com/openai/v1/chat/completions" \
            -H "Authorization: Bearer $GROQ_API_KEY" \
            -H "Content-Type: application/json" \
            -d "{\"model\":\"$model\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hello in one word.\"}],\"max_tokens\":5}" \
            --max-time 10 \
            -o "$outfile" 2>/dev/null)

        if [ "$http_code" -eq 200 ] && grep -q '"content"' "$outfile"; then
            rm -f "$outfile"
            return 0
        fi

        local error_msg
        error_msg=$(jq -r '.error.message // .error // "No details"' "$outfile" 2>/dev/null || cat "$outfile")
        rm -f "$outfile"
        echo "$error_msg"
        return 1
    }

    # Helper: pick a fallback chat model from the previously fetched models list
    find_fallback_model() {
        if [ -f "$MODELS_FILE" ] && command -v jq >/dev/null 2>&1; then
            # Pick the first active model whose ID suggests it's a chat/text model
            # (exclude whisper, playai, guard, and other obviously non‑chat ones).
            jq -r '.data[] | select(.active == true) | .id' "$MODELS_FILE" \
                | grep -ivE 'whisper|playai|guard|distil|tts' \
                | head -n 1
        fi
    }

    echo "==> Testing Groq API prompt (model: $MODEL) ..."
    ERROR_MSG=$(test_model "$MODEL") || true
    TEST_EXIT=$?

    if [ $TEST_EXIT -eq 0 ]; then
        echo "[ OK ] Groq API prompt response received"
    else
        if echo "$ERROR_MSG" | grep -qi "decommissioned"; then
            echo "[WARN] Configured model '$MODEL' is decommissioned."
            FALLBACK=$(find_fallback_model)
            if [ -n "$FALLBACK" ]; then
                echo "       Auto‑detected fallback model '$FALLBACK' from Groq API."
            else
                FALLBACK="llama-3.3-70b-versatile"
                echo "       Could not detect fallback dynamically; using last‑resort '$FALLBACK'."
            fi
            echo "       Trying fallback ..."
            FALLBACK_MSG=$(test_model "$FALLBACK") || true
            FALLBACK_EXIT=$?
            if [ $FALLBACK_EXIT -eq 0 ]; then
                echo "[ OK ] Fallback model works – your API key is valid."
                echo "       Please update 'model_name' in config.json to a supported model (e.g., $FALLBACK)."
            else
                echo "[FAIL] Fallback model also failed. Error: $FALLBACK_MSG"
                ERRORS=$((ERRORS+1))
            fi
        else
            echo "[FAIL] Groq API prompt test failed (HTTP error)"
            echo "       Error: $ERROR_MSG"
            ERRORS=$((ERRORS+1))
        fi
    fi

    # Cleanup
    rm -f "$MODELS_FILE"
fi

if [ $ERRORS -eq 0 ]; then
    echo "All checks passed"
else
    echo "$ERRORS check(s) failed"
fi