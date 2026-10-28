// s04 - Subagents
//
// Every file the agent reads stays in its context forever. Ask "which test
// framework does this repo use?" and the answer costs 30 file reads of
// context, though all you wanted was one word.
//
// A subagent is the same loop started with an EMPTY messages slice:
//
//	parent: task("find the test framework")
//	          └─ child: fresh messages, base tools, reads 30 files...
//	          └─ returns only its final text: "go test + testify"
//	parent context grows by one line, not 30 files.
//
// The child gets the base tools but no task tool (no recursive spawning) and
// no todo tool (the plan belongs to the parent).
//
// Run: go run ./agents/s04_subagent
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/cananoo/learn-agent-go/internal/kit"
)

var cfg = kit.Load()

var system = fmt.Sprintf(`You are a coding agent working in %s (%s).
Use the tools to inspect and change files. Prefer read_file/write_file/edit_file over shell for files.
For any task with more than two steps, first write a plan with the todo tool, keep exactly one item
in_progress, and update the list as you finish each step.
Use the task tool to delegate exploration or self-contained subtasks to a subagent; it returns only a summary,
which keeps your context small. Act, don't explain.`, cfg.Workdir, kit.Platform())

var subagentSystem = fmt.Sprintf(`You are a subagent working in %s (%s).
Complete the task you are given using the tools, then reply with a concise summary of what you found or did.
Your final message is all the parent agent will see.`, cfg.Workdir, kit.Platform())

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

// baseTools is what a subagent gets: it can act, but not plan or spawn.
var baseTools = tools[:4]

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
	tool("todo", "Replace the whole todo list. Use it to plan and track multi-step work.",
		map[string]any{"items": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":     str("Short id"),
					"text":   str("What to do"),
					"status": map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
				},
				"required": []string{"id", "text", "status"},
			},
		}}, "items"),
	tool("task", "Delegate a self-contained subtask to a subagent with a fresh context. Returns its final summary.",
		map[string]any{
			"description": str("Short label for the subtask, e.g. 'find test framework'"),
			"prompt":      str("Full instructions; the subagent sees nothing else"),
		}, "description", "prompt"),
}

// ----------------------------------------------------------------- todo list

type todoItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

// todoList is the agent's visible plan. Update validates, stores and renders it.
type todoList struct{ items []todoItem }

func (l *todoList) Update(items []todoItem) (string, error) {
	if len(items) > 20 {
		return "", errors.New("too many items (max 20); group them")
	}
	inProgress := 0
	for _, it := range items {
		switch it.Status {
		case "in_progress":
			inProgress++
		case "pending", "completed":
		default:
			return "", fmt.Errorf("item %s: invalid status %q", it.ID, it.Status)
		}
		if strings.TrimSpace(it.Text) == "" {
			return "", fmt.Errorf("item %s: empty text", it.ID)
		}
	}
	if inProgress > 1 {
		return "", errors.New("only one item can be in_progress at a time")
	}
	l.items = items
	return l.Render(), nil
}

func (l *todoList) Render() string {
	if len(l.items) == 0 {
		return "(no todos)"
	}
	var b strings.Builder
	done := 0
	for _, it := range l.items {
		mark := "[ ]"
		switch it.Status {
		case "in_progress":
			mark = "[>]"
		case "completed":
			mark = "[x]"
			done++
		}
		fmt.Fprintf(&b, "%s %s: %s\n", mark, it.ID, it.Text)
	}
	fmt.Fprintf(&b, "(%d/%d completed)", done, len(l.items))
	return b.String()
}

var todos = &todoList{}

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
	"todo": typed(func(in struct{ Items []todoItem }) (string, error) {
		return todos.Update(in.Items)
	}),
}

// task is registered in init because its handler calls runSubagent, which
// reads handlers: declaring it inline would be an initialization cycle.
func init() {
	handlers["task"] = typed(func(in struct{ Description, Prompt string }) (string, error) {
		return runSubagent(context.Background(), in.Description, in.Prompt)
	})
}

// subagentMaxRounds stops a confused child from looping forever.
const subagentMaxRounds = 30

// runSubagent is the agent loop again, with three differences: a fresh
// messages slice, the base tool set, and only the final text is returned.
func runSubagent(ctx context.Context, description, prompt string) (string, error) {
	kit.Info("[subagent] start: %s", description)
	messages := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))}
	for range subagentMaxRounds {
		resp, err := cfg.Client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     cfg.Model,
			MaxTokens: 16000,
			System:    []anthropic.TextBlockParam{{Text: subagentSystem}},
			Messages:  messages,
			Tools:     baseTools,
		})
		if err != nil {
			return "", err
		}
		messages = append(messages, resp.ToParam())

		var results []anthropic.ContentBlockParamUnion
		var text strings.Builder
		for _, block := range resp.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				text.WriteString(b.Text)
			case anthropic.ToolUseBlock:
				results = append(results, runTool(b, "task", "todo"))
			}
		}
		if resp.StopReason != anthropic.StopReasonToolUse {
			kit.Info("[subagent] done: %s", description)
			if text.Len() == 0 {
				return "(subagent returned no text)", nil
			}
			return text.String(), nil
		}
		messages = append(messages, anthropic.NewUserMessage(results...))
	}
	return "", fmt.Errorf("subagent hit %d rounds without finishing", subagentMaxRounds)
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
// Tools named in denied are refused, which is how the subagent is kept from
// spawning children or touching the parent's plan.
func runTool(b anthropic.ToolUseBlock, denied ...string) anthropic.ContentBlockParamUnion {
	raw := b.JSON.Input.Raw()
	h, ok := handlers[b.Name]
	if !ok || slices.Contains(denied, b.Name) {
		return anthropic.NewToolResultBlock(b.ID, "Error: unknown tool "+b.Name, true)
	}
	output, err := h(raw)
	if err != nil {
		output = "Error: " + err.Error()
	}
	kit.Tool(b.Name, raw, output)
	return anthropic.NewToolResultBlock(b.ID, output, err != nil)
}

// nagAfter is how many tool rounds without a todo update trigger a reminder.
const nagAfter = 3

// agentLoop is s02's loop plus one counter: rounds since the plan was touched.
func agentLoop(ctx context.Context, messages *[]anthropic.MessageParam) error {
	roundsSinceTodo := 0
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
		usedTodo := false
		for _, block := range resp.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				kit.Text(b.Text)
			case anthropic.ToolUseBlock:
				results = append(results, runTool(b))
				usedTodo = usedTodo || b.Name == "todo"
			}
		}

		switch resp.StopReason {
		case anthropic.StopReasonToolUse:
			if usedTodo {
				roundsSinceTodo = 0
				kit.Info("%s", todos.Render())
			} else if roundsSinceTodo++; roundsSinceTodo >= nagAfter {
				// A text block after the tool results is a cheap, append-only nudge.
				results = append(results, anthropic.NewTextBlock(
					"<reminder>You have not updated your todo list for a while. Update it now.</reminder>"))
				roundsSinceTodo = 0
			}
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
	kit.Info("s04 subagent | model %s | type q to quit", cfg.Model)
	var history []anthropic.MessageParam
	kit.REPL("s04", func(ctx context.Context, line string) error {
		history = append(history, anthropic.NewUserMessage(anthropic.NewTextBlock(line)))
		return agentLoop(ctx, &history)
	})
}
