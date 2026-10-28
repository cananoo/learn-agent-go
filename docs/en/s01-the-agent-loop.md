# s01 · The Agent Loop

> One loop and one tool. That is already a coding agent.

[Code](../../agents/s01_loop/main.go) · [Next: Tools →](s02-tools.md)

## The problem

A chat model can only produce text. It can *say* "run `go test`", but it cannot run it, see the failure, and try again. Every agent product solves this the same way.

## The idea

Give the model a tool it can call, run the call for it, send the result back, and repeat until it stops asking:

```
user ─▶ messages ─▶ model ─┬─ text only ──────────▶ done
                    ▲      └─ tool_use ─▶ run ─┐
                    └───── tool_result ◀───────┘
```

The model decides *what* to do. The loop only executes and reports. It never needs to know what the task is.

## The code

```go
for {
    resp, err := cfg.Client.Messages.New(ctx, anthropic.MessageNewParams{
        Model: cfg.Model, MaxTokens: 16000,
        System: []anthropic.TextBlockParam{{Text: system}},
        Messages: *messages, Tools: tools,
    })
    if err != nil { return err }
    *messages = append(*messages, resp.ToParam())   // 1. keep the assistant turn

    var results []anthropic.ContentBlockParamUnion
    for _, block := range resp.Content {
        if b, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
            results = append(results, anthropic.NewToolResultBlock(b.ID, runBash(...), false))
        }
    }
    if resp.StopReason != anthropic.StopReasonToolUse { return nil }   // 2. done?
    *messages = append(*messages, anthropic.NewUserMessage(results...)) // 3. feed back
}
```

Three details that are easy to get wrong:

1. **Append the assistant turn exactly as returned** (`resp.ToParam()`), before running tools. It may contain thinking blocks the API expects to see again unchanged.
2. **Every `tool_use` gets a `tool_result` with the same id**, even when the tool fails. A missing result makes the next request invalid.
3. **All results of one turn go in one user message.** The model may call several tools at once; answer them together.

## Stop reasons

| `stop_reason` | Meaning | What the loop does |
|---|---|---|
| `tool_use` | wants tools run | run them, continue |
| `end_turn` | finished | return |
| `max_tokens` | ran out of output budget | stop and say so |
| `refusal` | declined the request | stop and say so |

## Try it

```bash
go run ./agents/s01_loop
s01 >> create primes.go that prints the first 10 primes, then run it
```

Watch the model write a file through `bash`, run it, and fix it if the output is wrong. Nobody told the loop about files or compilers.

## Why this matters

Everything in the next five lessons keeps this loop as it is. Tools, planning, subagents, skills and compaction are all things you add *around* it.
