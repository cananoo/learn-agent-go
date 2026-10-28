# s06 · Context Compaction

> When the context fills up: save everything, summarize, and restart from the summary.

[← s05](s05-skills.md) · [Code](../../agents/s06_compact/main.go)

## The problem

Context is finite. A long session fills it with file dumps and command output. Requests get slower and more expensive, the model's attention spreads thin, and eventually the API rejects the request as too long.

## The tempting fix, and why not

"Just delete old tool results in place." It looks free, but:

- **Prompt caching works on prefixes.** Editing message 5 invalidates the cache for messages 5 through 500. You pay full price for the whole history on the next call.
- **Thinking blocks are bound to the history.** Recent Claude models return reasoning blocks that must be sent back unchanged, together with the history they were produced from. Rewriting earlier turns can make the API reject them.

So the rule for this agent is: **history is append-only.** When it gets too big, we don't edit it; we replace it all at once.

## The idea

```
after a round, if tokens(last request) > COMPACT_THRESHOLD   (or the user types /compact)
  1. save the full history to .transcripts/session-<time>.json
  2. ask the model, with no tools, for a hand-off summary
  3. history = [ one user message: summary + current todo list + transcript path ]
  4. continue; the new, short history is append-only again
```

## Measuring size for free

Every response reports how many tokens the request used. The last request is the best estimate of the next one:

```go
func contextTokens(u anthropic.Usage) int64 {
    return u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens + u.OutputTokens
}
```

Note the cache fields: with prompt caching, `InputTokens` alone only counts the *uncached* part and badly underestimates the context size.

## The summary prompt

The summary is written for "another engineer who will continue the work": goal and constraints, what is done (with file paths), what was in progress, and what failed. The current todo list is attached verbatim. A summary that loses the plan is how compacted agents start redoing finished work.

The full transcript stays on disk, and its path goes into the summary message, so the agent can `read_file` exact details later if it needs them.

## Try it

```bash
COMPACT_THRESHOLD=8000 go run ./agents/s06_compact
s06 >> read every file under agents/ and tell me how the lessons differ
```

With a low threshold you will see `(context at N tokens, compacting)` and the session continuing from the summary. `/compact` triggers it by hand.

## You have built the whole thing

s06 is the complete agent: loop, file tools, planning, subagents, skills and compaction, in about 650 lines of Go. Real products add permissions, streaming, MCP and a nicer UI. Those are next on the [roadmap](../../README.md#roadmap).
