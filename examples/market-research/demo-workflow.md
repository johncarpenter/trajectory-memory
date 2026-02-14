# Market Research Demo Workflow

This demonstrates how trajectory-memory improves research tasks over time.

## Setup

```bash
# Build the tool
cd trajectory-memory
go build -o trajectory-memory ./cmd/trajectory-memory

# Seed with example data
./trajectory-memory import examples/market-research/seed.jsonl

# Verify
./trajectory-memory list
```

## Demo Scenario

### Without Trajectory Memory

A new research task comes in:
> "Research the current state of AI code assistants and summarize key players, features, and market trends"

**Typical result:** Varies widely - might be thorough, might be shallow.

### With Trajectory Memory

**Step 1: Agent searches for past approaches**
```
trajectory_search({ "query": "AI code assistant market research", "min_score": 0.7 })
```

Returns:
```json
[
  {
    "session_id": "01JEXAMPLE001HIGHSCORE",
    "task_prompt": "Research the current state of AI code assistants...",
    "score": 0.92,
    "summary": "Comprehensive market research. Systematic approach: surveyed multiple authoritative sources..."
  }
]
```

**Step 2: Agent reviews high-scoring trajectory**
```
trajectory-memory show 01JEXAMPLE001
```

Learns:
- Start with project context (Read CLAUDE.md)
- Use multiple authoritative sources (Stack Overflow survey, vendor docs)
- Search for specific topics (enterprise adoption, features, market size)
- Structure output with sections (Executive Summary, Feature Matrix, Trends)

**Step 3: Agent executes with learned approach**
```
trajectory_start({ "task_prompt": "Research AI code assistants 2024", "tags": ["research", "market-analysis"] })
```

Now follows the proven pattern:
1. Read context files
2. Search authoritative sources
3. Compare multiple players
4. Structure findings
5. Include recommendations

**Step 4: After completion**
```
trajectory_stop({})
```

Agent generates summary, then:
```
trajectory_summarize({ "session_id": "...", "summary": "..." })
```

**Step 5: Score the result**
```
trajectory_score({ "session_id": "...", "score": 0.85, "notes": "Good structure, could add more on pricing" })
```

## What the Agent Learns

From high-scoring trajectories (0.8+):
- Always read CLAUDE.md first for context
- Use 5-8 targeted searches, not 1-2 generic ones
- Include authoritative sources (surveys, official docs)
- Structure output with executive summary + details
- Include comparison tables
- End with actionable recommendations

From low-scoring trajectories (0.0-0.4):
- Single generic search is insufficient
- Unstructured "stream of consciousness" output fails
- Missing source attribution reduces credibility
- No recommendations = not actionable

## Measuring Improvement

Over time, track:
1. Average score of research tasks
2. Consistency (standard deviation of scores)
3. Time to completion
4. User satisfaction

The hypothesis: With trajectory-memory, agents will:
- Converge on effective patterns faster
- Produce more consistent quality
- Learn domain-specific best practices

## Running in Claude Code

1. Start Claude Code in the `examples/market-research` directory
2. The CLAUDE.md file instructs the agent to use trajectory_search
3. Try: "Research the current state of AI code assistants"
4. Observe: Does the agent search for past approaches?
5. Compare output to examples in `output-examples/`
