# s04 · 子 Agent

> 同一个循环，从空历史开始。读 30 个文件，只交回一段话。

[← s03](s03-planning.md) · [代码](../../agents/s04_subagent/main.go) · [下一课：技能 →](s05-skills.md)

## 问题

工具返回的所有内容都会留在上下文里，直到会话结束。问一句“这个仓库用什么测试框架？”，Agent 读了 30 个文件才答出一个词。这 30 个文件之后每次请求都要付 token，还会挤占真正重要的内容。

## 思路

把探索放到**一个有自己 `messages` 的独立循环**里跑，只把最后的答案交回来：

```
父: task("找出测试框架")
      └─ 子: messages = [任务描述]      ← 全新上下文
         读 30 个文件、跑命令
         最终回复: "go test，表驱动，没用 testify"
父上下文只多了一条 tool_result，而不是 30 个文件。
```

子 Agent 不是什么新东西，就是 `agentLoop` 改了三处：

| | 父 | 子 |
|---|---|---|
| 历史 | 整个会话 | 只有任务描述 |
| 工具 | 全部，包括 `task` | 只有基础工具（没有 `task`、`todo`） |
| 返回 | 边做边打印 | 只返回最后一段文字 |

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
            return text.String(), nil   // 只有这段会回到父 Agent
        }
    }
}
```

## 护栏

- **不能递归。** 子 Agent 看不到 `task`，就算模型硬编出这个调用，`runTool(b, "task", "todo")` 也会拒绝。
- **不共享计划。** todo 列表属于父 Agent，子 Agent 改它只会添乱。
- **轮数上限**（30 轮），防止糊涂的子 Agent 无限循环。
- **任务描述要能独立成立。** 子 Agent 看不到父会话，所以工具描述里要求模型把完整指令写进去。

## Go 的一个细节：初始化循环

`task` 的 handler 调用 `runSubagent`，而 `runSubagent` 又读取 `handlers`。如果直接写在 `handlers` 字面量里，就构成初始化循环，Go 编译时会报错。放到 `init()` 里注册就解开了。

## 动手试试

```bash
go run ./agents/s04_subagent
s04 >> 用子 Agent 找出这个仓库里所有的 TODO 注释，然后给我总结一下
```

## 和 s03 相比改了什么

- 一个 `task` 工具，加一个 40 行左右的 `runSubagent`。
- `runTool` 多了一个拒绝列表参数。
