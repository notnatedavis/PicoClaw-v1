#!/usr/bin/env bash
# scripts/health-check.sh

# Health Checks:
# - binary presence and type (platform‑aware)
# - .env presence and loading
# - Ollama service reachability (both localhost and 127.0.0.1)
# - llama3.2:3b model availability (auto‑pull if missing)
# - model response test (simple prompt) WITH timing
# - Telegram bot token validity (getMe)
# - Optional: send a test message via Telegram if TEST_TELEGRAM_CHAT_ID is set
# - Port availability (18790) – if in use, automatically kills the rogue process.
# - Validation of config/agents/*, config/skills/*, pkg/* structure

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
        echo "    - Killing process(es) using port $port: $pids"
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
            echo "    [WARN] Some processes still hold port $port: $remaining"
        else
            echo "    [ OK ] Port $port is now free."
        fi
    else
        echo "    - No process found using port $port."
    fi
}

# ----- JSON validation helpers -----
validate_json_file() {
    local file="$1"
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "import json; json.load(open('$file'))" 2>/dev/null
    elif command -v jq >/dev/null 2>&1; then
        jq empty "$file" >/dev/null 2>&1
    else
        echo "    [WARN] No JSON validator (python3/jq). Skipping deep validation of $file"
        return 1
    fi
}

check_agent_json() {
    # Uses a single-line Python command to avoid indentation issues on all platforms.
    local file="$1"
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "import json, sys; d=json.load(open('$file')); missing=[k for k in ['name','system_prompt','tools'] if k not in d]; sys.exit(1 if missing else 0)" 2>/dev/null
    elif command -v jq >/dev/null 2>&1; then
        jq -e '.name and .system_prompt and .tools' "$file" >/dev/null 2>&1
    else
        echo "    [WARN] No JSON validator (python3/jq). Skipping key check for $file"
        return 1
    fi
}

# 1. binary presence and type check
if [ ! -f picoclaw ]; then
    echo ">  [FAIL] picoclaw binary not found"
    ERRORS=$((ERRORS+1))
else
    echo ">  [ OK ] picoclaw binary present"
    OS=$(detect_os)
    BINARY_TYPE=$(file -b picoclaw 2>/dev/null || echo "unknown")
    case "$OS" in
        mac)
            if [[ "$BINARY_TYPE" == *"Mach-O"* ]]; then
                echo ">  [ OK ] Binary is a macOS executable"
            else
                echo ">  [FAIL] Binary does not appear to be a macOS executable (type: $BINARY_TYPE)"
                ERRORS=$((ERRORS+1))
            fi
            ;;
        win)
            if [[ "$BINARY_TYPE" == *"PE32"* ]]; then
                echo ">  [ OK ] Binary is a Windows executable"
            else
                echo ">  [FAIL] Binary does not appear to be a Windows executable (type: $BINARY_TYPE)"
                ERRORS=$((ERRORS+1))
            fi
            ;;
        *)
            echo ">  [WARN] Unknown OS, skipping binary type check"
            ;;
    esac
fi

# 2. .env availability and loading
if [ -f .env ]; then
    set +e; set -a; source .env 2>/dev/null; set +a; set -e
    echo ">  [ OK ] .env loaded"
else
    echo ">  [FAIL] .env missing"
    ERRORS=$((ERRORS+1))
fi

# 3. Ollama service reachability – check both localhost and 127.0.0.1
echo ">  Checking Ollama service..."
OLLAMA_LOCAL="http://localhost:11434"
OLLAMA_IP="http://127.0.0.1:11434"
LOCAL_OK=0
IP_OK=0

if curl -s "$OLLAMA_LOCAL/api/tags" --max-time 5 >/dev/null 2>&1; then
    echo "    [ OK ] Ollama is reachable at localhost:11434"
    LOCAL_OK=1
else
    echo "    [WARN] Ollama not reachable at localhost:11434"
fi

if curl -s "$OLLAMA_IP/api/tags" --max-time 5 >/dev/null 2>&1; then
    echo "    [ OK ] Ollama is reachable at 127.0.0.1:11434"
    IP_OK=1
else
    echo "    [WARN] Ollama not reachable at 127.0.0.1:11434"
fi

if [ $LOCAL_OK -eq 0 ] && [ $IP_OK -eq 1 ]; then
    echo "    [WARN] 'localhost' does not resolve to 127.0.0.1. This may cause issues if the config uses 'localhost'."
    echo "           The default config now uses 127.0.0.1 to avoid this problem."
elif [ $LOCAL_OK -eq 0 ] && [ $IP_OK -eq 0 ]; then
    echo "    [FAIL] Ollama is not reachable at all. Is it installed and running? (scripts/setup.sh can help)"
    ERRORS=$((ERRORS+1))
fi

# 4. Required model (llama3.2:3b) presence and verification
MODEL="llama3.2:3b"
echo ">  Checking model $MODEL..."
MODEL_PRESENT=0
if command -v ollama >/dev/null 2>&1; then
    if ollama list 2>/dev/null | grep -q "$MODEL"; then
        echo "    [ OK ] Model $MODEL is already present."
        MODEL_PRESENT=1
    else
        echo "    [WARN] Model $MODEL not found. Attempting to pull..."
        if ollama pull "$MODEL"; then
            echo "    [ OK ] Pulled $MODEL successfully."
            MODEL_PRESENT=1
        else
            echo "    [FAIL] Could not pull $MODEL."
            ERRORS=$((ERRORS+1))
        fi
    fi
else
    echo "    [FAIL] ollama command not found. Cannot check model."
    ERRORS=$((ERRORS+1))
fi

# 5. Quick model response test WITH timing
if [ $MODEL_PRESENT -eq 1 ]; then
    echo ">  Testing a prompt with $MODEL (measuring speed)..."
    # Cross‑platform time measurement (seconds with decimal if supported)
    if date +%s.%N >/dev/null 2>&1; then
        # GNU date (Linux) – nanosecond precision
        START_TIME=$(date +%s.%N)
        RESPONSE=$(ollama run "$MODEL" "Say hello in one word." 2>/dev/null || true)
        END_TIME=$(date +%s.%N)
        ELAPSED=$(echo "$END_TIME - $START_TIME" | bc 2>/dev/null || echo "0")
    elif command -v perl >/dev/null 2>&1; then
        # macOS / BSD date lacks %N – use Perl for high precision
        START_TIME=$(perl -MTime::HiRes=time -e 'print time')
        RESPONSE=$(ollama run "$MODEL" "Say hello in one word." 2>/dev/null || true)
        END_TIME=$(perl -MTime::HiRes=time -e 'print time')
        ELAPSED=$(echo "$END_TIME - $START_TIME" | bc 2>/dev/null || echo "0")
    else
        # Fallback: integer seconds (less precise, but works everywhere)
        START_TIME=$(date +%s)
        RESPONSE=$(ollama run "$MODEL" "Say hello in one word." 2>/dev/null || true)
        END_TIME=$(date +%s)
        ELAPSED=$(( END_TIME - START_TIME ))
    fi

    if echo "$RESPONSE" | grep -qiE 'hello|hi|greetings'; then
        echo "    [ OK ] Model responded correctly: $RESPONSE"
    else
        echo "    [WARN] Model response was unexpected: $RESPONSE"
    fi

    # Display elapsed time
    if [ -n "$ELAPSED" ] && [ "$ELAPSED" != "0" ]; then
        # Convert to milliseconds if we have a fractional value
        if echo "$ELAPSED" | grep -q '\.'; then
            MS=$(echo "$ELAPSED * 1000" | bc | cut -d. -f1)
            echo "    - Response time: ${MS} ms"
        else
            echo "    - Response time: ${ELAPSED} seconds"
        fi
        # Warn if >5 seconds
        if (( $(echo "$ELAPSED > 5" | bc -l 2>/dev/null || echo 0) )); then
            echo "    [WARN] Response is very slow (>5s). Consider a smaller model or faster hardware."
        fi
    else
        echo "    - (could not measure time)"
    fi
fi

# 6. Telegram token and connectivity test
if [ -n "${TELEGRAM_BOT_TOKEN:-}" ]; then
    echo "[ OK ] Telegram bot token is set"
    echo ">  Testing Telegram Bot API (getMe) ..."
    HTTP_STATUS=$(curl -s -w "%{http_code}" \
        "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getMe" \
        --max-time 10 \
        -o /tmp/telegram_response.json 2>/dev/null)
    if [ "$HTTP_STATUS" -eq 200 ]; then
        if grep -q '"ok":true' /tmp/telegram_response.json; then
            BOT_USERNAME=$(jq -r '.result.username' /tmp/telegram_response.json 2>/dev/null || echo "unknown")
            echo "    [ OK ] Telegram bot token is valid (bot @$BOT_USERNAME)"
        else
            echo "    [FAIL] Telegram API returned ok:false. Check your token."
            ERRORS=$((ERRORS+1))
        fi
    else
        echo "    [FAIL] Telegram API unreachable or invalid token (HTTP $HTTP_STATUS)"
        ERRORS=$((ERRORS+1))
    fi
    rm -f /tmp/telegram_response.json

    # Optional: test sending a message if a test chat ID is provided
    if [ -n "${TEST_TELEGRAM_CHAT_ID:-}" ]; then
        echo ">  Sending test message to Telegram chat $TEST_TELEGRAM_CHAT_ID ..."
        SEND_RESPONSE=$(mktemp)
        HTTP_SEND=$(curl -s -w "%{http_code}" -X POST \
            "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
            -d "chat_id=$TEST_TELEGRAM_CHAT_ID&text=🧪 Health-check test from PicoClaw" \
            --max-time 10 \
            -o "$SEND_RESPONSE" 2>/dev/null)
        if [ "$HTTP_SEND" -eq 200 ] && grep -q '"ok":true' "$SEND_RESPONSE"; then
            echo "    [ OK ] Test message sent successfully."
        else
            echo "    [WARN] Test message failed (HTTP $HTTP_SEND). Check chat ID and that the bot has started a chat with that user."
        fi
        rm -f "$SEND_RESPONSE"
    fi
else
    echo "[FAIL] TELEGRAM_BOT_TOKEN not set"
    ERRORS=$((ERRORS+1))
fi

# ---- NEW: Configuration & source checks ----
echo ">  Validating configuration and source files..."

# Agents
if [ -d config/agents ]; then
    for f in config/agents/*.json; do
        [ -f "$f" ] || continue
        echo "    - Checking $f"
        if ! validate_json_file "$f"; then
            echo "      [FAIL] Invalid JSON"
            ERRORS=$((ERRORS+1))
        else
            echo "      [ OK ] Valid JSON"
            if ! check_agent_json "$f"; then
                echo "      [FAIL] Missing required keys (name, system_prompt, tools)"
                ERRORS=$((ERRORS+1))
            else
                echo "      [ OK ] Required keys present"
            fi
        fi
    done
else
    echo "    [FAIL] config/agents/ directory missing"
    ERRORS=$((ERRORS+1))
fi

# Skills (optional but warn if present and invalid)
if [ -d config/skills ]; then
    for f in config/skills/*.json; do
        [ -f "$f" ] || continue
        echo "    - Checking $f"
        if ! validate_json_file "$f"; then
            echo "      [FAIL] Invalid JSON"
            ERRORS=$((ERRORS+1))
        else
            echo "      [ OK ] Valid JSON"
        fi
    done
else
    echo "    [INFO] No config/skills/ directory (optional)."
fi

# Source files in pkg/
echo "    - Checking pkg/ structure..."
REQUIRED_PKG=(
    "pkg/agent/agent.go"
    "pkg/agent/pipeline_llm.go"
    "pkg/agent/registry.go"
    "pkg/gateway/gateway.go"
)
for file in "${REQUIRED_PKG[@]}"; do
    if [ -f "$file" ]; then
        echo "      [ OK ] $file"
    else
        echo "      [FAIL] $file MISSING"
        ERRORS=$((ERRORS+1))
    fi
done

# 7. Port availability check (gateway uses 18790) – automatically kill rogue processes
echo ">  Checking port 18790 availability..."
OS=$(detect_os)
PID=""
case "$OS" in
    win)
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
    echo "    [WARN] Port 18790 is already in use by process $PID. Killing it now..."
    kill_process_on_port 18790
else
    echo "    [ OK ] Port 18790 is free."
fi

if [ $ERRORS -eq 0 ]; then
    echo "All checks passed"
else
    echo "$ERRORS check(s) failed"
fi