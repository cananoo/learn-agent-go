# s02 · 工具

> 加一项能力 = 加一个 schema 和一个 handler，循环不用动。

[← s01](s01-the-agent-loop.md) · [代码](../../agents/s02_tools/main.go) · [下一课：计划 →](s03-planning.md)

## 问题

只有 `bash` 的时候，模型只能用 `sed` 和 heredoc 改文件。能用，但迟早出事：引号转义错了、正则匹配到两处、路径跑出了项目目录。shell 适合当兜底，不适合当主要接口。

## 思路

给模型专门的、输入有结构的工具，所有调用都走同一张分发表：

```go
var handlers = map[string]handler{
    "bash":       ...,
    "read_file":  ...,
    "write_file": ...,
    "edit_file":  ...,
}
```

循环只做一件事：查 `handlers[block.Name]`。加第 5 个工具也不用碰循环。

## 用泛型写有类型的 handler

每个工具收到的是原始 JSON。一个很小的泛型适配器把它解码成结构体，handler 就很好读：

```go
func typed[T any](fn func(T) (string, error)) handler {
    return func(raw string) (string, error) {
        var in T
        if err := json.Unmarshal([]byte(raw), &in); err != nil {
            return "", fmt.Errorf("invalid input: %w", err)
        }
        return fn(in)
    }
}

"edit_file": typed(func(in struct {
    Path    string
    OldText string `json:"old_text"`
    NewText string `json:"new_text"`
}) (string, error) {
    return editFile(in.Path, in.OldText, in.NewText)
}),
```

## 错误是给模型看的输入，不是崩溃

handler 返回的 error 会变成 `is_error: true` 的 `tool_result`，模型读到后会自己改正。所以错误信息要**写给模型看**：

```go
case n == 0: return "", errors.New("old_text not found; read the file and copy the text exactly")
case n > 1:  return "", fmt.Errorf("old_text appears %d times; include more context to make it unique", n)
```

`edit_file` 要求恰好匹配一处，和 Claude Code 的编辑工具规则一样，能避免经典的“改错了地方”问题。

## 不许跑出工作目录

`safePath` 把每个路径都相对工作目录解析，凡是逃出去的（比如 `../../etc/passwd`）一律拒绝。只有几行，但每个文件工具都应该有。

## 动手试试

```bash
go run ./agents/s02_tools
s02 >> 读一下 go.mod，在 require 那行上面加一行注释说明这个 SDK 是干什么的
```

## 和 s01 相比改了什么

- `tools` 从 1 个变成 4 个；`handlers` 把名字映射到函数。
- `runTool` 负责查表、包装错误、打印。
- 循环体一模一样。
