#!/bin/bash
# trajectory-memory hook script
# Reads Claude Code hook payload from stdin and forwards to trajectory-memory ingestion socket
# This script is fire-and-forget - errors are silently ignored to not block CC

SOCKET_PATH="${TM_SOCKET_PATH:-/tmp/trajectory-memory.sock}"

PAYLOAD=$(cat)

# Only proceed if socket exists
if [ -S "$SOCKET_PATH" ]; then
    curl -s -X POST --unix-socket "$SOCKET_PATH" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD" \
        --max-time 1 \
        http://localhost/step > /dev/null 2>&1 || true
fi
