#!/bin/bash
# ============================================================================
# Real CLI Agent Wrapper — Process Detection + Real Probe Delegation
# ============================================================================
# Pattern: install real package → rename binary → this wrapper takes its place
# - For --version (probe): delegates to the real binary
# - Otherwise: sleeps so /proc/PID/comm shows this script's name
# ============================================================================
# Usage: create this file as /usr/local/bin/<agent-name>
#        and place the real binary at /usr/local/bin/<agent-name>-real
# ============================================================================
set -e

REAL_NAME="$(basename "$0")"
REAL_BIN="/usr/local/bin/${REAL_NAME}-real"

# If invoked with --version or -v or version, delegate to real binary
for arg in "$@"; do
    case "$arg" in
        --version|-v|version)
            if [ -x "$REAL_BIN" ]; then
                exec "$REAL_BIN" "$@"
            else
                echo "${REAL_NAME}: real binary not found at ${REAL_BIN}" >&2
                exit 1
            fi
            ;;
    esac
done

# Otherwise: sleep for process detection
exec sleep 999999
