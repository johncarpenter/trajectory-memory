#!/bin/bash
# Seed the trajectory-memory database with market research examples

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SEED_FILE="$SCRIPT_DIR/seed.jsonl"

echo "Seeding trajectory-memory with market research examples..."

# Import the seed data
trajectory-memory import "$SEED_FILE"

echo ""
echo "Seeded sessions:"
trajectory-memory list --limit 10

echo ""
echo "Try searching for past approaches:"
echo "  trajectory-memory search 'market research'"
echo "  trajectory-memory search 'AI code assistant'"
echo "  trajectory-memory show 01JEXAMPLE001"
