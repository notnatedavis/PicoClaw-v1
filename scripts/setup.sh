#!/usr/bin/env bash
# scripts/setup.sh

# One‑command setup :
#   - creates directories
#   - copies .env.example → .env if missing
#   - auto‑detects OS/architecture and copies the matching binary from binaries/
#   - renames it to 'picoclaw' in the project root
#   - sets environment variables
#   - validates agent/skills configs and pkg/ structure
#   - configures Ollama to use a custom models directory (platform‑aware)
#   - ensures Ollama + the llama3.2:3b model (supports tools) are ready

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ----- Helper Functions -----
detect_os() {
    case "$(uname -s)" in
        Darwin)  echo "mac" ;;
        MINGW*|MSYS*|CYGWIN*)  echo "win" ;;
        *)       echo "unknown" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "x86" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             echo "unknown" ;;
    esac
}

set_ollama_models_path() {
    local os="$1"
    case "$os" in
        mac)
            export OLLAMA_MODELS="$HOME/Desktop/everything/coding/other/ollama/models/"
            ;;
        win)
            export OLLAMA_MODELS="$USERPROFILE\\Desktop\\everything\\coding\\other\\ollama\\models\\"
            ;;
        *)
            # fallback: keep default behaviour
            export OLLAMA_MODELS=""
            ;;
    esac
    echo "    - OLLAMA_MODELS set to: $OLLAMA_MODELS"
}

stop_ollama_if_running() {
    if pgrep -x ollama >/dev/null 2>&1; then
        echo "    - Stopping running Ollama service..."
        pkill ollama 2>/dev/null || true
        sleep 1
    fi
}

# ----- JSON validation helper (requires python3 or jq) -----
validate_json_file() {
    local file="$1"
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "import json; json.load(open('$file'))" 2>/dev/null
    elif command -v jq >/dev/null 2>&1; then
        jq empty "$file" >/dev/null 2>&1
    else
        echo "    [WARN] No JSON validator found (install python3 or jq). Skipping validation of $file"
        return 1
    fi
}

check_agent_json() {
    local file="$1"
    local errors=0
    # Check required keys: name, system_prompt, tools
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "
import json, sys
with open('$file') as f:
    data = json.load(f)
    for key in ['name', 'system_prompt', 'tools']:
        if key not in data:
            print(f'Missing key: {key}')
            sys.exit(1)
" 2>/dev/null || errors=1
    elif command -v jq >/dev/null 2>&1; then
        jq -e '.name and .system_prompt and .tools' "$file" >/dev/null 2>&1 || errors=1
    fi
    return $errors
}

echo ">  Setting up PicoClaw environment..."

# 1. prepare .env file
if [ ! -f .env ]; then
    if [ -f .env.example ]; then
        echo "    - Creating .env from .env.example. Please edit it with your keys"
        cp .env.example .env
        echo "      Run: nano .env   when ready"
    else
        echo "    [ERROR] No .env.example found. Cannot continue"
        exit 1
    fi
fi

# source .env robustly (ignore errors, export everything)
set +e; set -a; source .env 2>/dev/null; set +a; set -e

# 2. create required directories
mkdir -p logs workspace/agent-sessions/default

# 3. remove stale PID if present (should not be running during setup)
rm -f picoclaw.pid

# 4. detect OS and architecture
OS=$(detect_os)
ARCH=$(detect_arch)

if [ "$OS" = "unknown" ] || [ "$ARCH" = "unknown" ]; then
    echo "    [ERROR] Unsupported OS/architecture: $(uname -s) / $(uname -m)"
    echo "            Only macOS (Darwin) and Windows (Git Bash / MSYS2) are supported."
    exit 1
fi

# Build the source binary filename
EXT=""
if [ "$OS" = "win" ]; then
    EXT=".exe"
fi
SOURCE_BINARY="binaries/picoclaw-binary-${OS}-${ARCH}${EXT}"

if [ ! -f "$SOURCE_BINARY" ]; then
    echo "    [ERROR] Binary not found: $SOURCE_BINARY"
    echo "            Please ensure the binaries are placed in the 'binaries/' directory."
    exit 1
fi

echo "    - Detected OS: $OS, Architecture: $ARCH"
echo "    - Copying $SOURCE_BINARY to ./picoclaw ..."

# Copy the binary to the root (overwrites any existing one)
cp "$SOURCE_BINARY" picoclaw
chmod +x picoclaw

# 5. ensure PICOCLAW_CONFIG is set for this session
export PICOCLAW_CONFIG="$REPO_ROOT/config/config.json"
echo "    - PICOCLAW_CONFIG set to $PICOCLAW_CONFIG"

# ---- NEW: Validate config/agents, config/skills, and pkg/ structure ----
echo ""
echo ">  Validating configuration files and source structure..."

FAILURES=0

# --- agents ---
if [ -d config/agents ]; then
    agent_count=0
    for f in config/agents/*.json; do
        [ -f "$f" ] || continue
        ((agent_count++))
        echo "    - Checking agent config: $f"
        if ! validate_json_file "$f"; then
            echo "      [FAIL] Invalid JSON in $f"
            FAILURES=$((FAILURES+1))
        else
            echo "      [ OK ] Valid JSON"
        fi
        if ! check_agent_json "$f"; then
            echo "      [FAIL] Missing required keys (name, system_prompt, tools)"
            FAILURES=$((FAILURES+1))
        else
            echo "      [ OK ] Required keys present"
        fi
    done
    if [ $agent_count -eq 0 ]; then
        echo "    [WARN] No agent configs found in config/agents/"
        FAILURES=$((FAILURES+1))
    else
        echo "    [ OK ] Found $agent_count agent config(s)."
    fi
else
    echo "    [ERROR] config/agents/ directory missing. Cannot proceed."
    exit 1
fi

# --- skills ---
if [ -d config/skills ]; then
    skill_count=0
    for f in config/skills/*.json; do
        [ -f "$f" ] || continue
        ((skill_count++))
        echo "    - Checking skill config: $f"
        if ! validate_json_file "$f"; then
            echo "      [FAIL] Invalid JSON in $f"
            FAILURES=$((FAILURES+1))
        else
            echo "      [ OK ] Valid JSON"
        fi
    done
    if [ $skill_count -gt 0 ]; then
        echo "    [ OK ] Found $skill_count skill config(s)."
    else
        echo "    [INFO] No skill configs found in config/skills/ (optional)."
    fi
else
    echo "    [INFO] config/skills/ directory not found (optional)."
fi

# --- pkg/ source files ---
echo "    - Checking pkg/ source files..."
REQUIRED_PKG_FILES=(
    "pkg/agent/agent.go"
    "pkg/agent/pipeline_llm.go"
    "pkg/agent/registry.go"
    "pkg/gateway/gateway.go"
)
for file in "${REQUIRED_PKG_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "      [ OK ] $file exists"
    else
        echo "      [FAIL] $file MISSING"
        FAILURES=$((FAILURES+1))
    fi
done

# Final verdict
if [ $FAILURES -gt 0 ]; then
    echo ""
    echo "[WARN] $FAILURES validation issue(s) found. Some features may not work correctly."
    echo "       Please fix them and re-run setup.sh or continue at your own risk."
else
    echo "    [ OK ] All configuration and source file validations passed."
fi

# ---- Proceed with Ollama setup ----
echo ""
echo ">  Configuring Ollama..."

# Set the platform‑specific models directory
set_ollama_models_path "$OS"

# Ensure any running Ollama is stopped so we can restart with our env
stop_ollama_if_running

# Install Ollama if missing
if ! command -v ollama &> /dev/null; then
    echo "    - Installing Ollama..."
    curl -fsSL https://ollama.com/install.sh | sh
else
    echo "    - Ollama already installed."
fi

# Start Ollama in the background with our custom OLLAMA_MODELS
echo "    - Starting Ollama serve with custom model path..."
nohup ollama serve > /dev/null 2>&1 &
sleep 2

# Quick connectivity test
if ! curl -s http://localhost:11434/api/tags --max-time 5 >/dev/null 2>&1; then
    echo "    [WARN] Ollama service did not start. Check logs and try manually."
else
    echo "    [ OK ] Ollama service is reachable."
fi

# Pull the required model (llama3.2:3b supports tools/function‑calling)
MODEL="llama3.2:3b"
echo ">  Checking for model $MODEL ..."
if ollama list 2>/dev/null | grep -q "$MODEL"; then
    echo "    - Model $MODEL already available."
else
    echo "    - Pulling $MODEL (this may take a while on first run)..."
    ollama pull "$MODEL"
fi

# Verify the model responds
echo ">  Verifying $MODEL with a quick test..."
RESPONSE=$(ollama run "$MODEL" "Say hello in one word." 2>/dev/null || true)
if echo "$RESPONSE" | grep -qiE 'hello|hi|greetings'; then
    echo "    [ OK ] Model response: $RESPONSE"
else
    echo "    [WARN] Model test returned unexpected output: $RESPONSE"
    echo "           The model may still work, but the test prompt didn't produce the expected keyword."
fi

echo ">  Setup complete"
echo "    Next: bash scripts/start.sh"