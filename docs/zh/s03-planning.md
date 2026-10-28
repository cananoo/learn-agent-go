# s03 · 用 Todo 列表做计划

> 一个对外部世界毫无作用的工具，却能让 Agent 明显变强。

[← s02](s02-tools.md) · [代码](../../agents/s03_todo/main.go) · [下一课：子 Agent →](s04-subagents.md)

## 问题

让它改五处，它改了三处。长任务里模型会跑偏：修第 2 步时被一个报错吸引过去，就再也没回到第 4 步。上下文里唯一提到第 4 步的，只剩历史很靠前的一句话。

## 思路

把计划变成**看得见的状态**。`todo` 工具每次调用都整体替换列表：

```
todo([{id:"1", text:"读配置",   status:"completed"},
      {id:"2", text:"加参数",   status:"in_progress"},
      {id:"3", text:"跑测试",   status:"pending"}])
```

工具结果会把渲染好的列表回显出来，于是当前计划总是出现在上下文的末尾，也就是模型最关注的位置：

```
[x] 1: 读配置
[>] 2: 加参数
[ ] 3: 跑测试
(1/3 completed)
```

## 两条规则

1. **最多一个 `in_progress`。** 校验会拒绝两个，逼模型做完（或明确暂停）一件事再开始下一件。
2. **列表久不更新就提醒。** 连续 3 轮工具调用都没碰 `todo`，循环就在工具结果后面追加一段文字：

```go
} else if roundsSinceTodo++; roundsSinceTodo >= nagAfter {
    results = append(results, anthropic.NewTextBlock(
        "<reminder>You have not updated your todo list for a while. Update it now.</reminder>"))
    roundsSinceTodo = 0
}
```

提醒是**追加**的，从不改写之前的消息。只追加能让 prompt cache 保持有效，历史也和模型看到的完全一致。

## 动手试试

```bash
go run ./agents/s03_todo
s03 >> 在 calc/ 下写一个能加减乘的小命令行工具，带测试，然后跑测试
```

每次更新计划后都会打印出来，能看到它一项项打勾。

## 和 s02 相比改了什么

- 一个新工具，加一个 50 行左右的 `todoList` 类型。
- 循环里多一个计数器。
- 系统提示词里多一句“先做计划”。
