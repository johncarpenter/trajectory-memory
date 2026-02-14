#!/bin/bash
# Seed the trajectory-memory database with meeting summary examples

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SEED_FILE="$SCRIPT_DIR/seed.jsonl"

echo "Seeding trajectory-memory with meeting summary examples..."

trajectory-memory import "$SEED_FILE"

echo ""
echo "Seeded sessions:"
trajectory-memory search "meeting"

echo ""
echo "Try viewing a high-scoring approach:"
echo "  trajectory-memory show 01JMEETING001"
