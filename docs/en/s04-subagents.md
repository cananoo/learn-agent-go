# s04 · Subagents

> The same loop, started with an empty history. Explore 30 files, return one paragraph.

[← s03](s03-planning.md) · [Code](../../agents/s04_subagent/main.go) · [Next: Skills →](s05-skills.md)

## The problem

Everything a tool returns stays in the context for the rest of the session. Ask "which test framework does this repo use?" and the agent reads 30 files to answer in one word. Those 30 files now cost tokens on every later request and crowd out what matters.

## The idea

Run the exploration in a **separate loop with its own `messages`**, and hand back only its final answer:

```
parent: task("find the test framework")
          └─ child: messages = [prompt]      ← fresh context
             reads 30 files, runs commands
             final text: "go test, table-driven, no testify"
parent context grows by one tool_result, not 30 files.
```

A subagent is not a new kind of thing. It is `agentLoop` with three differences:

| | parent | child |
|---|---|---|
| history | the whole session | just the task prompt |
| tools | all, including `task` | base tools only (no `task`, no `todo`) |
| returns | prints as it goes | only its last text |

```go
func runSubagent(ctx context.Context, description, prompt string) (string, error) {
    messages := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))}
    for range subagentMaxRounds {
        resp, err := cfg.Client.Messages.New(ctx, anthropic.MessageNewParams{
            System: []anthropic.TextBlockParam{{Text: subagentSystem}},
            Messages: messages, Tools: baseTools, /* ... */
        })
        ...
        if resp.StopReason != anthropic.StopReasonToolUse {
            return text.String(), nil   // only this crosses back to the parent
        }
    }
}
```

## Guard rails

- **No recursion.** The child does not see `task`, and `runTool(b, "task", "todo")` refuses it even if a model invents the call.
- **No shared plan.** The todo list belongs to the parent; a child rewriting it would be confusing.
- **A round limit** (30) so a confused child cannot loop forever.
- **The prompt must stand alone.** The child sees nothing of the parent conversation, so the tool description tells the model to pass full instructions.

## A Go detail: init cycles

The `task` handler calls `runSubagent`, which reads `handlers`. Declaring it inside the `handlers` literal is an initialization cycle, which Go rejects at compile time. Registering it in `init()` breaks the cycle.

## Try it

```bash
go run ./agents/s04_subagent
s04 >> use a subagent to find every TODO comment in this repo, then summarize them for me
```

## What changed from s03

- A `task` tool and a 40-line `runSubagent`.
- `runTool` takes a deny list.
