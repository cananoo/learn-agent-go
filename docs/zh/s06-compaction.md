# s06 · 上下文压缩

> 上下文快满时：全部存档，写一份摘要，从摘要重新开始。

[← s05](s05-skills.md) · [代码](../../agents/s06_compact/main.go)

## 问题

上下文是有限的。长会话里塞满了文件内容和命令输出，请求越来越慢、越来越贵，模型注意力被摊薄，最后 API 直接以“太长”拒绝请求。

## 看起来省事的做法，以及为什么不这么做

“直接把旧的工具结果原地删掉。”看似没有代价，但是：

- **Prompt cache 是按前缀生效的。** 改了第 5 条消息，第 5 到第 500 条的缓存全部失效，下一次请求要为整段历史付全价。
- **thinking 块和历史是绑定的。** 新一代 Claude 模型返回的推理块需要原样带回，并且要和产生它们的那段历史一起带回。改写之前的轮次，可能导致 API 拒绝这些块。

所以这个 Agent 的规矩是：**历史只追加，不修改。** 太大了也不去改它，而是一次性整体替换。

## 思路

```
每一轮之后，如果 tokens(上一次请求) > COMPACT_THRESHOLD   （或者用户输入 /compact）
  1. 把完整历史存到 .transcripts/session-<时间>.json
  2. 不带工具地请模型写一份交接摘要
  3. 历史 = [ 一条 user 消息：摘要 + 当前 todo 列表 + 存档路径 ]
  4. 继续干活；新的、很短的历史又是只追加的了
```

## 免费测量上下文大小

每次响应都会报告这次请求用了多少 token，上一次请求就是下一次的最好估计：

```go
func contextTokens(u anthropic.Usage) int64 {
    return u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens + u.OutputTokens
}
```

注意缓存相关的字段：开启 prompt caching 后，`InputTokens` 只统计**未命中缓存**的部分，单看它会严重低估上下文大小。

## 摘要提示词

摘要写给“接手继续干活的另一位工程师”：目标和约束、做完了什么（带文件路径）、正在做什么、哪些尝试失败了。当前 todo 列表原样附上。摘要把计划弄丢了，压缩后的 Agent 就会开始重做已经完成的工作。

完整存档留在磁盘上，路径写进摘要消息里，Agent 之后需要细节时可以用 `read_file` 查。

## 动手试试

```bash
COMPACT_THRESHOLD=8000 go run ./agents/s06_compact
s06 >> 读一遍 agents/ 下的所有文件，告诉我每一课有什么区别
```

阈值调低后，会看到 `(context at N tokens, compacting)`，然后会话从摘要继续。输入 `/compact` 可以手动触发。

## 你已经写完了整个 Agent

s06 就是完整的 Agent：循环、文件工具、计划、子 Agent、技能、压缩，大约 650 行 Go。真正的产品还会加上权限、流式输出、MCP 和更好的界面，这些都在[路线图](../../README_zh.md#路线图)里。
