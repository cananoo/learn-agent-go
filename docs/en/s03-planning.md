# s03 · Planning with a Todo List

> A tool that changes nothing in the world, and makes the agent noticeably better.

[← s02](s02-tools.md) · [Code](../../agents/s03_todo/main.go) · [Next: Subagents →](s04-subagents.md)

## The problem

Ask for five changes and you get three. On long tasks the model drifts: it fixes step 2, gets absorbed by an error, and never comes back to step 4. Nothing in the context says step 4 exists anymore except a sentence far up in the history.

## The idea

Make the plan **visible state**. The `todo` tool replaces the whole list each time it is called:

```
todo([{id:"1", text:"read config",  status:"completed"},
      {id:"2", text:"add flag",     status:"in_progress"},
      {id:"3", text:"run tests",    status:"pending"}])
```

The result echoes the rendered list back, so the current plan is always near the end of the context where the model pays the most attention:

```
[x] 1: read config
[>] 2: add flag
[ ] 3: run tests
(1/3 completed)
```

## Two rules

1. **At most one item `in_progress`.** Validation rejects two. It forces the model to finish (or explicitly pause) one thing before starting another.
2. **A reminder when the list goes stale.** If 3 tool rounds pass without a `todo` call, the loop appends a text block after the tool results:

```go
} else if roundsSinceTodo++; roundsSinceTodo >= nagAfter {
    results = append(results, anthropic.NewTextBlock(
        "<reminder>You have not updated your todo list for a while. Update it now.</reminder>"))
    roundsSinceTodo = 0
}
```

The reminder is *appended*, never edited into earlier messages. Appending keeps the prompt cache valid and leaves the history exactly as the model saw it.

## Try it

```bash
go run ./agents/s03_todo
s03 >> create a tiny CLI in calc/ that adds, subtracts and multiplies, with tests, and run the tests
```

You will see the plan printed after each update, and items ticked off as it goes.

## What changed from s02

- One new tool and a 50-line `todoList` type.
- One counter in the loop.
- One sentence in the system prompt asking it to plan.
