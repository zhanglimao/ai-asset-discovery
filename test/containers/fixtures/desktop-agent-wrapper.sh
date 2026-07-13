#!/bin/bash
# ============================================================================
# Realistic Agent Wrapper — for agents that cannot be installed as packages
# ============================================================================
# Usage: install as /usr/local/bin/<agent-name>
#   - --version / -v: outputs realistic version string
#   - otherwise: exec sleep for process detection (comm = this binary's name)
# ============================================================================
NAME="$(basename "$0")"

for arg in "$@"; do
    case "$arg" in
        --version|-v|version)
            case "$NAME" in
                windsurf)        echo "Windsurf IDE v1.2.0" ;;
                chatgpt-desktop) echo "ChatGPT Desktop v1.2025.153" ;;
                Claude)          echo "Claude Desktop v0.46.0" ;;
                WorkBuddy)       echo "WorkBuddy v3.5.0" ;;
                Marvis)          echo "Marvis v2.1.0" ;;
                Qclaw)           echo "Qclaw v1.8.0" ;;
                QoderWork)       echo "QoderWork v2.3.0" ;;
                TRAE)            echo "TRAE Work v4.0.0" ;;
                doubao)          echo "豆包专业版 v3.2.0" ;;
                Kimi)            echo "Kimi Work v2.5.0" ;;
                DuMate)          echo "DuMate v1.6.0" ;;
                jieyue)          echo "阶跃AI v1.4.0" ;;
                AutoClaw)        echo "AutoClaw v2.0.0" ;;
                lobster)         echo "lobsterAI v1.3.0" ;;
                Cola)            echo "Cola v2.1.0" ;;
                Alice)           echo "Alice v3.0.0" ;;
                niuma)           echo "牛马AI v1.7.0" ;;
                Manus)           echo "Manus v1.5.0" ;;
                Fellou)          echo "Fellou v2.0.0" ;;
                Genspark)        echo "Genspark v1.9.0" ;;
                AutoGLM)         echo "AutoGLM v1.3.0" ;;
                dify)            echo "Dify v0.15.0" ;;
                coze)            echo "Coze v2.0.0" ;;
                computer-use)    echo "Claude Computer Use v0.8.0" ;;
                yuanqi)          echo "腾讯元器 v1.2.0" ;;
                dingtalk)        echo "钉钉AI助理 v2.0.0" ;;
                bolt)            echo "Bolt.new v1.0.0" ;;
                replit)          echo "Replit Agent v1.0.0" ;;
                *)               echo "${NAME} v1.0.0" ;;
            esac
            exit 0
            ;;
    esac
done

# Keep /proc/PID/comm = this binary's name (via hardlink to sleep)
exec sleep 999999
