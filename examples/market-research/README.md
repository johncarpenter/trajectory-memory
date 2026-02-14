# Example A: Market Research

This example demonstrates trajectory-memory for research and synthesis tasks.

## The Task

Research a topic and produce a structured summary with key findings.

## Example Prompts

**Primary task:**
```
Research the current state of AI code assistants and summarize key players, features, and market trends
```

**Variations (for testing search):**
- "Analyze the competitive landscape for developer productivity tools"
- "Research AI coding tools and their adoption in enterprise"
- "Compare GitHub Copilot, Cursor, and Claude Code features"

## What Makes a Good Trajectory?

**High-scoring approaches (0.8-1.0):**
- Systematic: defines scope, then researches, then synthesizes
- Uses multiple sources (web search, official docs, recent articles)
- Structures output clearly (players, features matrix, trends, outlook)
- Cites sources and acknowledges limitations
- Produces actionable insights, not just facts

**Medium-scoring approaches (0.5-0.7):**
- Completes the task but misses depth
- Single source of information
- Output is informative but unstructured
- No clear methodology visible

**Low-scoring approaches (0.0-0.4):**
- Shallow research (one quick search)
- Generic or outdated information
- No structure or synthesis
- Hallucinates or makes unsupported claims

## Running the Example

1. **Seed the database with example trajectories:**
   ```bash
   cd examples/market-research
   ./seed.sh
   ```

2. **View seeded sessions:**
   ```bash
   trajectory-memory list
   trajectory-memory search "market research"
   ```

3. **Run a new research task in Claude Code:**
   - Start Claude Code in this directory
   - Say: "Research the current state of AI code assistants"
   - The agent should use `trajectory_search` to find past approaches
   - Score the result when complete

4. **Compare approaches:**
   ```bash
   trajectory-memory show <session-id>
   ```

## Files

- `seed.jsonl` - Pre-scored example trajectories
- `seed.sh` - Script to import seed data
- `output-examples/` - Example outputs at different quality levels
