#!/bin/bash
# trajectory-memory hook script
# Reads Claude Code hook payload from stdin and forwards to trajectory-memory ingestion socket
# This script is fire-and-forget - errors are silently ignored to not block CC

# Find project root by looking for markers (same logic as Go code)
find_project_root() {
    local dir="$PWD"
    while [[ "$dir" != "/" ]]; do
        if [[ -d "$dir/.git" || -f "$dir/CLAUDE.md" || -d "$dir/.claude" ]]; then
            echo "$dir"
            return
        fi
        dir="$(dirname "$dir")"
    done
    # No marker found, use current directory
    echo "$PWD"
}

# Compute socket path from project root
get_socket_path() {
    local project_root="$1"
    # SHA256 hash, first 8 chars (matches Go implementation)
    local hash=$(echo -n "$project_root" | shasum -a 256 | cut -c1-8)
    echo "/tmp/trajectory-memory-${hash}.sock"
}

# Allow override via environment variable
if [[ -n "$TM_SOCKET_PATH" ]]; then
    SOCKET_PATH="$TM_SOCKET_PATH"
else
    PROJECT_ROOT=$(find_project_root)
    SOCKET_PATH=$(get_socket_path "$PROJECT_ROOT")
fi

PAYLOAD=$(cat)

# Only proceed if socket exists
if [ -S "$SOCKET_PATH" ]; then
    curl -s -X POST --unix-socket "$SOCKET_PATH" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD" \
        --max-time 1 \
        http://localhost/step > /dev/null 2>&1 || true
fi
