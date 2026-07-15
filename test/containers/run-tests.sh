#!/bin/bash
# ============================================================================
# AI Asset Discovery - Container Test Suite (v2)
# ============================================================================
# Design principles:
#   1. Each test container focuses on ONE agent category (no coupling)
#   2. Real packages/binaries where possible; realistic wrappers otherwise
#   3. Probe delegates to real binaries; processes use exec sleep for comm detection
#   4. Multi-agent: real mixed environment, validates dedup + multi-category harmony
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

# ============================================================================
# run_test: Run a single container test
# ============================================================================
# Usage: run_test <name> <image> <expected-agent-names> <process-spec>
#
# process-spec: comma-separated entries, one per agent:
#   "NAME"              → spawn "NAME 999999 &" (matches name_patterns + cmd_patterns)
#   "real:PY_CODE"      → spawn "python3 -c import time; PY_CODE; time.sleep(99999) &"
#   "code:SCAN_MODE"    → VS Code extension scan (no process — uses hardcoded scan dirs)
# ============================================================================
run_test() {
  local name="$1" image="$2" expected="$3" procs="$4"
  echo ""
  printf "${CYAN}[TEST]${NC} %s\n" "$name"
  local out="$RESULTS_DIR/${name}.json"

  local iscript="set -e; "
  IFS=',' read -ra PA <<< "$procs"
  for spec in "${PA[@]}"; do
    if [[ "$spec" == real:* ]]; then
      local real_code="${spec#real:}"
      iscript+="python3 -c \"import time; ${real_code}; time.sleep(99999)\" & "
    elif [[ "$spec" == code:* ]]; then
      :  # no process needed — IDE extension scan via hardcoded scan paths
    else
      # Explicit /usr/local/bin/ path: sleep hardlinks for process comm detection
      iscript+="/usr/local/bin/${spec} 999999 & "
    fi
  done
  iscript+="sleep 2; "
  # Discovery runs with /opt/agents/bin first in PATH for real probe output
  iscript+="PATH=/opt/agents/bin:/usr/local/bin:/usr/bin:/bin /usr/local/bin/discovery --rules /rules --pretty=false --output /results/${name}.json 2>&1 || true"

  docker run --rm --name "aid-${name}" \
    --cap-add=SYS_PTRACE \
    -v "$DISCOVERY_BIN:/usr/local/bin/discovery:ro" \
    -v "$RULES_DIR:/rules:ro" \
    -v "$RESULTS_DIR:/results" \
    --entrypoint /bin/bash \
    "$image" -c "$iscript" > "$RESULTS_DIR/${name}.log" 2>&1 || true

  if [ ! -f "$out" ]; then
    fail "$name" "No JSON output"
    return
  fi

  local all_found=true details=""
  IFS=',' read -ra EXP <<< "$expected"
  for e in "${EXP[@]}"; do
    if python3 -c "import json; d=json.load(open('$out')); exit(0 if any(a['name']=='$e' for a in d.get('agents',[])) else 1)" 2>/dev/null; then
      details+=" ✓$e"
    else
      details+=" ✗$e"
      all_found=false
    fi
  done

  local summary
  summary=$(python3 -c "import json; d=json.load(open('$out')); print(f\"total={d['summary']['total_agents']} conf={d['summary']['confirmed_agents']} poss={d['summary']['possible_agents']} ghost={d['summary']['ghost_agents']}\")" 2>/dev/null || echo "parse-error")

  if $all_found; then
    pass "$name" "$details | $summary"
  else
    fail "$name" "$details | $summary"
  fi
}

# ============================================================================
# Main
# ============================================================================
echo "============================================"
echo " AI Asset Discovery - Container Tests v2"
echo "============================================"

[ -x "$DISCOVERY_BIN" ] || { echo "ERROR: No discovery binary at $DISCOVERY_BIN"; exit 1; }

# ── Build all images ──────────────────────────────────────────
echo ""
echo "=== Building Images ==="

for nm in cli-agents desktop-assistants browser-agents agent-platforms ide-extensions llm-sdk multi-agent google-agents; do
  printf "${YELLOW}[BUILD]${NC} %s ... " "$nm"
  # Use cache if image already exists
  if docker build --network=host -q -f "$SCRIPT_DIR/dockerfiles/Dockerfile.${nm}" -t "ai-discovery-test/${nm}:latest" "$PROJECT_DIR" > /dev/null 2>&1; then
    echo "OK"
  else
    echo "FAIL"
    exit 1
  fi
done

# ── Run tests ─────────────────────────────────────────────────
echo ""
echo "=== Running Tests ==="

# 1. CLI Agents (aider real pip, claude-code/gemini-cli simulated probe, 18 agents total)
run_test "cli-agents" "ai-discovery-test/cli-agents:latest" \
  "aider,claude-code,gemini-cli,openclaw,hermes-agent,opencode,reasonix,kiro,omp,grok-cli,open-interpreter,openspec,devin,codex-cli,qwen-code,openhands,goose,warp" \
  "aider,claude-code,gemini-cli,openclaw,hermes,opencode,reasonix,kiro-cli,omp,grok,interpreter,openspec,devin,codex,qwen,openhands,goose,warp"

# 2. Desktop Assistants (18 agents: Windsurf + ChatGPT + Claude + 15 desktop agents)
run_test "desktop-assistants" "ai-discovery-test/desktop-assistants:latest" \
  "windsurf,chatgpt-desktop,claude-desktop,workbuddy,marvis,qclaw,qoderwork,trae-work,doubao-pro,kimi-work,dumate,jieyue-ai,autoclaw,lobsterai,cola,alice,niuma-ai" \
  "windsurf,chatgpt-desktop,Claude,WorkBuddy,Marvis,Qclaw,QoderWork,TRAE,doubao,Kimi,DuMate,jieyue,AutoClaw,lobster,Cola,Alice,niuma"

# 3. Browser Agents (5 agents)
run_test "browser-agents" "ai-discovery-test/browser-agents:latest" \
  "manus,claude-computer-use,fellou,genspark,autoglm" \
  "Manus,computer-use,Fellou,Genspark,AutoGLM"

# 4. Agent Platforms & Web Tools (6 agents: 4 process + 2 file-only)
run_test "agent-platforms" "ai-discovery-test/agent-platforms:latest" \
  "dify,coze,dingtalk-ai,tencent-yuanqi,bolt-new,replit-agent" \
  "dify,coze,dingtalk,yuanqi"

# 5. IDE Extensions (12+ VS Code extensions — real package.json files, no process)
run_test "ide-extensions" "ai-discovery-test/ide-extensions:latest" \
  "github-copilot,github-copilot-agent,cline,continue,roo-code,augment,amazon-q-developer,tabnine,sourcegraph-cody,lingma,trae,codebuddy,qoder,gemini-code-assist" \
  "code:--extensions-dir"

# 6. LLM SDK (real pip + npm packages + process sim)
run_test "llm-sdk" "ai-discovery-test/llm-sdk:latest" \
  "llm-sdk-detected,langchain,llamaindex,autogen,crewai" \
  "real:k1='langchain';k2='llama_index';k3='autogen';k4='crewai',langchain,llama_index,autogen,crewai"

# 7. Multi-Agent (real CLI + real SDK + IDE extensions in one environment)
run_test "multi-agent" "ai-discovery-test/multi-agent:latest" \
  "aider,claude-code,hermes-agent,opencode,reasonix,kiro,omp,grok-cli,open-interpreter,openspec,llm-sdk-detected,langchain,autogen,crewai,github-copilot,cline,tabnine,codebuddy,qoder" \
  "aider,claude,hermes,opencode,reasonix,kiro-cli,omp,grok,interpreter,openspec,real:k1='langchain';k2='autogen';k3='crewai',code:--extensions-dir"

# 8. Google AI Agents (Antigravity + Gemini Code Assist + ADK + Mariner + NotebookLM)
run_test "google-agents" "ai-discovery-test/google-agents:latest" \
  "google-antigravity,google-antigravity-ide,gemini-code-assist,google-adk,google-project-mariner,google-notebooklm" \
  "antigravity,antigravity-cli,adk,mariner,notebooklm,code:--extensions-dir"

# ── Summary ────────────────────────────────────────────────────
echo ""
echo "============================================"
echo " SUMMARY"
echo "============================================"
printf "Passed: ${GREEN}%d${NC}  Failed: ${RED}%d${NC}\n" $PASSED $FAILED
for nm in cli-agents desktop-assistants browser-agents agent-platforms ide-extensions llm-sdk multi-agent google-agents; do
  s="${TEST_RESULTS[$nm]:-SKIP}"
  if [ "$s" = "PASS" ]; then
    printf "  ${GREEN}✓${NC} %s\n" "$nm"
  else
    printf "  ${RED}✗${NC} %s\n" "$nm"
  fi
done
echo ""
echo "Logs: $RESULTS_DIR/"
exit $FAILED
