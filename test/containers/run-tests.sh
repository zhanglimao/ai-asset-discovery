#!/bin/bash
# ============================================================================
# AI Asset Discovery - Container Test Suite
# ============================================================================
# Process model: each agent binary is a hardlink to /bin/sleep.
#   /proc/PID/comm = binary name (correctly matches name_patterns)
#   /proc/PID/cmdline = "binary_name sleep_arg" (correctly matches cmd_patterns)
# No more exec -a — hardlinks give the kernel the right filename for comm.
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
DISCOVERY_BIN="$SCRIPT_DIR/discovery"
RULES_DIR="$PROJECT_DIR/rules"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
mkdir -p "$RESULTS_DIR"

PASSED=0; FAILED=0
declare -A TEST_RESULTS

pass() { printf "${GREEN}[PASS]${NC} %s %s\n" "$1" "$2"; PASSED=$((PASSED+1)); TEST_RESULTS["$1"]="PASS"; }
fail() { printf "${RED}[FAIL]${NC} %s %s\n" "$1" "$2"; FAILED=$((FAILED+1)); TEST_RESULTS["$1"]="FAIL"; }

# Usage: run_test <name> <image> <expected-names> <process-spec>
# process-spec: comma-separated entries
#   "NAME"        → spawn "NAME 999999 &"           (hardlink → /bin/sleep, comm=NAME)
#   "real:PY:CODE" → spawn "python3 -c CODE &"       (real Python with SDK imports)
#   "code:--arg"  → special: spawn VS Code extension scan context (no process)
run_test() {
  local name="$1" image="$2" expected="$3" procs="$4"
  echo ""; printf "${CYAN}[TEST]${NC} %s\n" "$name"
  local out="$RESULTS_DIR/${name}.json"

  local iscript="set -e; "
  IFS=',' read -ra PA <<< "$procs"
  for spec in "${PA[@]}"; do
    if [[ "$spec" == real:* ]]; then
      local real="${spec#real:}"
      local py_code="${real#python3:}"
      # keywords are comma-separated Python identifiers to appear in cmdline
      iscript+="python3 -c \"import time; ${py_code}; time.sleep(99999)\" & "
    elif [[ "$spec" == code:* ]]; then
      :  # no process — IDE extension scan only
    else
      iscript+="${spec} 999999 & "
    fi
  done
  iscript+="sleep 2; "
  iscript+="/usr/local/bin/discovery --rules /rules --pretty=false --output /results/${name}.json 2>&1 || true"

  docker run --rm --name "aid-${name}" \
    --cap-add=SYS_PTRACE \
    -v "$DISCOVERY_BIN:/usr/local/bin/discovery:ro" \
    -v "$RULES_DIR:/rules:ro" \
    -v "$RESULTS_DIR:/results" \
    --entrypoint /bin/bash \
    "$image" -c "$iscript" > "$RESULTS_DIR/${name}.log" 2>&1 || true

  if [ ! -f "$out" ]; then
    fail "$name" "No JSON output"; return
  fi

  local all_found=true details=""
  IFS=',' read -ra EXP <<< "$expected"
  for e in "${EXP[@]}"; do
    if python3 -c "import json; d=json.load(open('$out')); exit(0 if any(a['name']=='$e' for a in d.get('agents',[])) else 1)" 2>/dev/null; then
      details+=" ✓$e"
    else
      details+=" ✗$e"; all_found=false
    fi
  done

  local summary
  summary=$(python3 -c "import json; d=json.load(open('$out')); print(f\"total={d['summary']['total_agents']} conf={d['summary']['confirmed_agents']} poss={d['summary']['possible_agents']} ghost={d['summary']['ghost_agents']}\")" 2>/dev/null || echo "parse-error")

  if $all_found; then pass "$name" "$details | $summary"
  else fail "$name" "$details | $summary"; fi
}

echo "============================================"
echo " AI Asset Discovery - Container Tests"
echo "============================================"

[ -x "$DISCOVERY_BIN" ] || { echo "ERROR: No discovery binary"; exit 1; }

# Build all images
echo ""; echo "=== Building Images ==="
for nm in aider openclaw claude-code gemini-cli llm-sdk ide-extensions desktop-apps multi-agent; do
  printf "${YELLOW}[BUILD]${NC} %s ... " "$nm"
  if docker inspect "ai-discovery-test/${nm}:latest" >/dev/null 2>&1; then
    echo "CACHED"
  elif docker build --network=host -q -f "$SCRIPT_DIR/dockerfiles/Dockerfile.${nm}" -t "ai-discovery-test/${nm}:latest" "$PROJECT_DIR" > /dev/null 2>&1; then
    echo "OK"
  else
    echo "FAIL"
  fi
done

echo ""; echo "=== Running Tests ==="

# ── CLI Agents (real packages + hardlinks) ────────────────────
run_test "aider"          "ai-discovery-test/aider:latest"          "aider"                              "aider"
run_test "claude-code"    "ai-discovery-test/claude-code:latest"    "claude-code"                        "claude-code"
run_test "gemini-cli"     "ai-discovery-test/gemini-cli:latest"     "gemini-cli"                         "gemini-cli"
run_test "openclaw"       "ai-discovery-test/openclaw:latest"       "openclaw"                           "openclaw"

# ── LLM SDK Detection (real Python) ───────────────────────────
run_test "llm-sdk"        "ai-discovery-test/llm-sdk:latest"        "llm-sdk-detected,langchain,llamaindex,autogen,crewai" "real:python3:k1='langchain';k2='llama_index';k3='autogen';k4='crewai'"

# ── IDE Extensions ────────────────────────────────────────────
run_test "ide-extensions" "ai-discovery-test/ide-extensions:latest" "github-copilot,cline,continue,amazon-q-developer,tabnine,sourcegraph-cody,roo-code,augment,trae,lingma,codebuddy,qoder" "code:--extensions-dir"

# ── Desktop & Personal Assistants ──────────────────────────────
# All 32 agents as hardlinks → comm/cmdline both match correctly
run_test "desktop-apps"   "ai-discovery-test/desktop-apps:latest"   "windsurf,chatgpt-desktop,claude-desktop,workbuddy,marvis,qclaw,qoderwork,trae-work,doubao-pro,kimi-work,dumate,jieyue-ai,autoclaw,lobsterai,cola,alice,niuma-ai,manus,fellou,genspark,autoglm,dify,coze,qwen-code,openhands,goose,warp,devin,codex-cli,tabnine,dingtalk-ai,tencent-yuanqi" "windsurf,chatgpt-desktop,Claude,WorkBuddy,Marvis,Qclaw,QoderWork,TRAE,doubao,Kimi,DuMate,jieyue,AutoClaw,lobster,Cola,Alice,niuma,Manus,Fellou,Genspark,AutoGLM,dify,coze,qwen,openhands,goose,warp,devin,codex,tabnine,dingtalk,yuanqi"

# ── Mixed multi-agent ──────────────────────────────────────────
run_test "multi-agent"    "ai-discovery-test/multi-agent:latest"    "aider,claude-code,llm-sdk-detected,langchain,autogen,crewai,github-copilot,cline,tabnine,codebuddy,qoder" "aider,claude-code,real:python3:k1='langchain';k2='autogen';k3='crewai',code:--extensions-dir"

echo ""; echo "============================================"
echo " SUMMARY"
echo "============================================"
printf "Passed: ${GREEN}%d${NC}  Failed: ${RED}%d${NC}\n" $PASSED $FAILED
for nm in aider claude-code gemini-cli openclaw llm-sdk ide-extensions desktop-apps multi-agent; do
  s="${TEST_RESULTS[$nm]:-SKIP}"
  [ "$s" = "PASS" ] && printf "  ${GREEN}✓${NC} %s\n" "$nm" || printf "  ${RED}✗${NC} %s\n" "$nm"
done
echo "Logs: $RESULTS_DIR/"
exit $FAILED
