# Example C: Meeting Summary

This example demonstrates trajectory-memory for document synthesis - specifically creating executive summaries with action items from meeting transcripts.

## The Task

Transform a meeting transcript into an actionable executive summary.

## Example Prompts

**Primary task:**
```
Review the product planning meeting transcript and create an executive summary with action items
```

**Variations:**
- "Summarize the customer advisory board session highlighting key themes"
- "Create a sprint retrospective summary with improvement actions"
- "Extract action items and decisions from the meeting notes"

## What Makes a Good Trajectory?

**High-scoring approaches (0.8-1.0):**
- Multi-pass reading (context first, extraction second)
- Structured output (executive summary, decisions, action items table)
- Specific action items with owners and deadlines
- Identifies risks and dependencies
- Captures the "so what" - why decisions were made

**Medium-scoring approaches (0.5-0.7):**
- Captures main points but lacks structure
- Action items present but missing owners or dates
- Decisions listed but rationale missing
- No risk identification

**Low-scoring approaches (0.0-0.4):**
- Just a paragraph restating topics
- Missing action items
- No structure for executives
- Reader still needs to read original transcript

## Sample Transcripts

- `docs/product-planning-meeting.txt` - 45-minute product roadmap meeting
- `docs/customer-feedback-session.txt` - 60-minute customer advisory board

## Running the Example

1. **Seed the database:**
   ```bash
   trajectory-memory import examples/meeting-summary/seed.jsonl
   ```

2. **View seeded sessions:**
   ```bash
   trajectory-memory search "meeting summary"
   trajectory-memory show 01JMEETING001
   ```

3. **Run in Claude Code:**
   - Start Claude Code in this directory
   - Try: "Create an executive summary of the product planning meeting"
   - The CLAUDE.md instructs searching for past approaches

4. **Compare outputs:**
   - `output-examples/high-score-output.md` - Structured, actionable
   - `output-examples/low-score-output.md` - Vague, not actionable

## Key Learnings from Trajectories

What the agent learns from high-scoring sessions:

1. **Two-pass reading** - Don't try to summarize on first read
2. **Table format for action items** - Owner | Deadline | Status
3. **Separate decisions from discussion** - Leaders want outcomes
4. **Risk section** - Proactively flag concerns
5. **Executive summary up front** - TL;DR in first 30 seconds

## Metrics

Track improvement with:
- Completeness: Are all action items captured?
- Specificity: Do action items have owners + dates?
- Structure: Can an exec understand in 2 minutes?
- Accuracy: Does summary match transcript?
