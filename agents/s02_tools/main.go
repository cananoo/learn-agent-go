// s02 - Tools
//
// The loop from s01 does not change at all. Adding a capability means adding
// two things: a tool definition the model can see, and a handler in a
// dispatch map. That's it.
//
//	handlers := map[string]handler{
//	    "bash":       ...,
//	    "read_file":  ...,
//	    "write_file": ...,
//	    "edit_file":  ...,
//	}
//
// Dedicated file tools beat "cat/sed via bash": inputs are structured, paths
// are checked to stay inside the workspace, and edits are exact.
//
// Run: go run ./agents/s02_tools
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/cananoo/learn-agent-go/internal/kit"
)

var cfg = kit.Load()

var system = fmt.Sprintf(`You are a coding agent working in %s (%s).
Use the tools to inspect and change files. Prefer read_file/write_file/edit_file over shell for files.
Act, don't explain.`, cfg.Workdir, kit.Platform())

// ---------------------------------------------------------------- tool table

// tool builds a tool definition from a JSON-schema "properties" map.
func tool(name, desc string, props map[string]any, required ...string) anthropic.ToolUnionParam {
	return anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
		Name:        name,
		Description: anthropic.String(desc),
		InputSchema: anthropic.ToolInputSchemaParam{Properties: props, Required: required},
	}}
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

var tools = []anthropic.ToolUnionParam{
	tool("bash", "Run a shell command in the working directory.",
		map[string]any{"command": str("The command to run")}, "command"),
	tool("read_file", "Read a text file. Optionally only the first `limit` lines.",
		map[string]any{"path": str("File path, relative to the workspace"), "limit": map[string]any{"type": "integer"}}, "path"),
	tool("write_file", "Create or overwrite a file with the given content.",
		map[string]any{"path": str("File path"), "content": str("Full file content")}, "path", "content"),
	tool("edit_file", "Replace old_text with new_text in a file. old_text must appear exactly once.",
		map[string]any{"path": str("File path"), "old_text": str("Exact text to replace"), "new_text": str("Replacement text")},
		"path", "old_text", "new_text"),
}

// handler turns raw JSON tool input into a result. A returned error is sent
// back to the model as an is_error tool result, so it can correct itself.
type handler func(raw string) (string, error)

// typed adapts a function taking a decoded struct into a handler.
func typed[T any](fn func(T) (string, error)) handler {
	return func(raw string) (string, error) {
		var in T
		if err := json.Unmarshal([]byte(raw), &in); err != nil {
			return "", fmt.Errorf("invalid input: %w", err)
		}
		return fn(in)
	}
}

var handlers = map[string]handler{
	"bash": typed(func(in struct{ Command string }) (string, error) {
		return runBash(in.Command), nil
	}),
	"read_file": typed(func(in struct {
		Path  string
		Limit int
	}) (string, error) {
		return readFile(in.Path, in.Limit)
	}),
	"write_file": typed(func(in struct{ Path, Content string }) (string, error) {
		return writeFile(in.Path, in.Content)
	}),
	"edit_file": typed(func(in struct {
		Path    string
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}) (string, error) {
		return editFile(in.Path, in.OldText, in.NewText)
	}),
}

// ------------------------------------------------------------ implementations

func runBash(command string) string {
	for _, bad := range []string{"rm -rf /", "sudo ", "shutdown", "reboot", "mkfs", ":(){"} {
		if strings.Contains(command, bad) {
			return "Error: dangerous command blocked"
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	argv := kit.ShellCommand(command)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cfg.Workdir
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "Error: timeout (120s)"
	}
	result := strings.TrimSpace(string(out))
	if err != nil {
		result += "\n(exit: " + err.Error() + ")"
	}
	if result == "" {
		return "(no output)"
	}
	return truncate(result)
}

func truncate(s string) string {
	if len(s) > 50000 {
		return s[:50000] + "\n...(truncated)"
	}
	return s
}

// safePath resolves p inside the workspace and refuses anything that escapes it.
func safePath(p string) (string, error) {
	full := p
	if !filepath.IsAbs(full) {
		full = filepath.Join(cfg.Workdir, p)
	}
	full = filepath.Clean(full)
	rel, err := filepath.Rel(cfg.Workdir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace: %s", p)
	}
	return full, nil
}

func readFile(path string, limit int) (string, error) {
	full, err := safePath(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if limit > 0 && limit < len(lines) {
		lines = append(lines[:limit], fmt.Sprintf("...(%d more lines)", len(lines)-limit))
	}
	return truncate(strings.Join(lines, "\n")), nil
}

func writeFile(path, content string) (string, error) {
	full, err := safePath(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(content), path), nil
}

func editFile(path, oldText, newText string) (string, error) {
	full, err := safePath(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	switch n := strings.Count(string(data), oldText); {
	case oldText == "":
		return "", errors.New("old_text is empty")
	case n == 0:
		return "", errors.New("old_text not found; read the file and copy the text exactly")
	case n > 1:
		return "", fmt.Errorf("old_text appears %d times; include more context to make it unique", n)
	}
	updated := strings.Replace(string(data), oldText, newText, 1)
	if err := os.WriteFile(full, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return "edited " + path, nil
}

// ----------------------------------------------------------------- the loop

// runTool looks up a handler and builds the tool_result block.
func runTool(b anthropic.ToolUseBlock) anthropic.ContentBlockParamUnion {
	raw := b.JSON.Input.Raw()
	h, ok := handlers[b.Name]
	if !ok {
		return anthropic.NewToolResultBlock(b.ID, "Error: unknown tool "+b.Name, true)
	}
	output, err := h(raw)
	if err != nil {
		output = "Error: " + err.Error()
	}
	kit.Tool(b.Name, raw, output)
	return anthropic.NewToolResultBlock(b.ID, output, err != nil)
}

// agentLoop is unchanged from s01 except that tools go through runTool.
func agentLoop(ctx context.Context, messages *[]anthropic.MessageParam) error {
	for {
		resp, err := cfg.Client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     cfg.Model,
			MaxTokens: 16000,
			System:    []anthropic.TextBlockParam{{Text: system}},
			Messages:  *messages,
			Tools:     tools,
		})
		if err != nil {
			return err
		}
		*messages = append(*messages, resp.ToParam())

		var results []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				kit.Text(b.Text)
			case anthropic.ToolUseBlock:
				results = append(results, runTool(b))
			}
		}

		switch resp.StopReason {
		case anthropic.StopReasonToolUse:
			*messages = append(*messages, anthropic.NewUserMessage(results...))
		case anthropic.StopReasonMaxTokens:
			kit.Info("(stopped: hit max_tokens)")
			return nil
		case anthropic.StopReasonRefusal:
			kit.Info("(the model declined this request)")
			return nil
		default:
			return nil
		}
	}
}

func main() {
	kit.Info("s02 tools | model %s | type q to quit", cfg.Model)
	var history []anthropic.MessageParam
	kit.REPL("s02", func(ctx context.Context, line string) error {
		history = append(history, anthropic.NewUserMessage(anthropic.NewTextBlock(line)))
		return agentLoop(ctx, &history)
	})
}
