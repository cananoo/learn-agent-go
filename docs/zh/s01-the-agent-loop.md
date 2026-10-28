# s01 · Agent 循环

> 一个循环加一个工具，就已经是一个编程 Agent。

[代码](../../agents/s01_loop/main.go) · [下一课：工具 →](s02-tools.md)

## 问题

聊天模型只能输出文字。它可以**说**“运行一下 `go test`”，但没法真的去跑、看到报错、再改一遍。所有 Agent 产品解决这个问题的办法都一样。

## 思路

给模型一个可以调用的工具，替它执行，把结果送回去，循环往复，直到它不再要求调用工具：

```
用户 ─▶ messages ─▶ 模型 ─┬─ 只有文字 ───────────▶ 结束
                    ▲     └─ tool_use ─▶ 执行 ─┐
                    └───── tool_result ◀───────┘
```

**做什么**由模型决定，循环只负责执行和汇报。循环本身根本不需要知道任务是什么。

## 代码

```go
for {
    resp, err := cfg.Client.Messages.New(ctx, anthropic.MessageNewParams{
        Model: cfg.Model, MaxTokens: 16000,
        System: []anthropic.TextBlockParam{{Text: system}},
        Messages: *messages, Tools: tools,
    })
    if err != nil { return err }
    *messages = append(*messages, resp.ToParam())   // 1. 先把助手回复存进历史

    var results []anthropic.ContentBlockParamUnion
    for _, block := range resp.Content {
        if b, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
            results = append(results, anthropic.NewToolResultBlock(b.ID, runBash(...), false))
        }
    }
    if resp.StopReason != anthropic.StopReasonToolUse { return nil }   // 2. 结束了吗？
    *messages = append(*messages, anthropic.NewUserMessage(results...)) // 3. 结果喂回去
}
```

三个很容易写错的细节：

1. **助手回复要原样存进历史**（`resp.ToParam()`），而且要在执行工具之前存。里面可能有 thinking 块，API 要求下一轮原样带回。
2. **每个 `tool_use` 都必须有一个 id 相同的 `tool_result`**，工具失败也一样。少一个，下一次请求就不合法。
3. **同一轮的所有结果放进同一条 user 消息。** 模型可能一次调用多个工具，要一起回答。

## stop_reason

| `stop_reason` | 含义 | 循环怎么做 |
|---|---|---|
| `tool_use` | 要求执行工具 | 执行，继续 |
| `end_turn` | 说完了 | 返回 |
| `max_tokens` | 输出额度用完 | 停下并提示 |
| `refusal` | 拒绝了请求 | 停下并提示 |

## 动手试试

```bash
go run ./agents/s01_loop
s01 >> 写一个 primes.go 打印前 10 个质数，然后运行它
```

你会看到模型通过 `bash` 写文件、运行，输出不对还会自己修。循环里没有任何关于文件或编译器的代码。

## 为什么重要

后面五课都不改这个循环。工具、计划、子 Agent、技能、压缩，全都是在它**外面**加东西。
