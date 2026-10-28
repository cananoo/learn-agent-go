# s02 · Tools

> Adding a capability means adding a schema and a handler. The loop does not change.

[← s01](s01-the-agent-loop.md) · [Code](../../agents/s02_tools/main.go) · [Next: Planning →](s03-planning.md)

## The problem

With only `bash`, the model edits files with `sed` and heredocs. That works until it doesn't: quoting breaks, a regex matches twice, a path escapes the project. Shell is a fine escape hatch but a poor primary interface.

## The idea

Give the model dedicated tools with structured inputs, and route every call through one dispatch map:

```go
var handlers = map[string]handler{
    "bash":       ...,
    "read_file":  ...,
    "write_file": ...,
    "edit_file":  ...,
}
```

The loop just looks up `handlers[block.Name]`. Adding tool #5 never touches the loop.

## Typed handlers with generics

Each tool receives raw JSON. A tiny generic adapter decodes it into a struct so handlers stay readable:

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

## Errors are input, not crashes

A returned error becomes a `tool_result` with `is_error: true`. The model reads it and corrects itself. That is why error messages are written *for the model*:

```go
case n == 0: return "", errors.New("old_text not found; read the file and copy the text exactly")
case n > 1:  return "", fmt.Errorf("old_text appears %d times; include more context to make it unique", n)
```

`edit_file` insists on exactly one match. It is the same rule Claude Code's edit tool uses, and it prevents the classic "replaced the wrong occurrence" bug.

## Stay inside the workspace

`safePath` resolves every path against the working directory and rejects anything that escapes it (`../../etc/passwd`). It is a few lines and it belongs in every file tool.

## Try it

```bash
go run ./agents/s02_tools
s02 >> read go.mod, then add a comment above the require line explaining what the SDK is for
```

## What changed from s01

- `tools` grew from 1 to 4 entries; `handlers` maps names to functions.
- `runTool` does lookup, error wrapping and printing.
- The loop body is identical.
