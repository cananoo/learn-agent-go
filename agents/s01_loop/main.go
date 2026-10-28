// s01 - The Agent Loop
//
// The whole secret of a coding agent fits in one loop:
//
//	for {
//	    resp := model(messages, tools)
//	    if resp has no tool calls { return }      // the model is done
//	    results := run each tool call
//	    messages = append(messages, resp, results) // feed results back
//	}
//
// The model decides what to do; the loop only executes and reports back.
// With a single bash tool this is already a working coding agent.
//
// Run: go run ./agents/s01_loop
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/cananoo/learn-agent-go/internal/kit"
)

var cfg = kit.Load()

var system = fmt.Sprintf(`You are a coding agent working in %s (%s).
Use the bash tool to inspect and change files. Act, don't explain.`, cfg.Workdir, kit.Platform())

// One tool: the model sends {"command": "..."} and gets the output back.
var tools = []anthropic.ToolUnionParam{{OfTool: &anthropic.ToolParam{
	Name:        "bash",
	Description: anthropic.String("Run a shell command in the working directory and return its output."),
	InputSchema: anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"command": map[string]any{"type": "string", "description": "The command to run"},
		},
		Required: []string{"command"},
	},
}}}

// runBash executes a command with a timeout and returns combined output.
// The blocklist is a seatbelt for demos, not a sandbox: run agents in a
// directory you are willing to lose.
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
	if len(result) > 50000 {
		result = result[:50000] + "\n...(truncated)"
	}
	return result
}

// agentLoop is the core pattern. It keeps calling the model until the model
// stops asking for tools. messages is updated in place so the conversation
// carries over to the next user prompt.
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
		// Append the assistant turn exactly as returned (including any
		// thinking blocks) before we run tools.
		*messages = append(*messages, resp.ToParam())

		var results []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				kit.Text(b.Text)
			case anthropic.ToolUseBlock:
				var in struct {
					Command string `json:"command"`
				}
				output := ""
				if err := json.Unmarshal([]byte(b.JSON.Input.Raw()), &in); err != nil {
					output = "Error: invalid input: " + err.Error()
				} else {
					output = runBash(in.Command)
				}
				kit.Tool("bash", in.Command, output)
				results = append(results, anthropic.NewToolResultBlock(b.ID, output, false))
			}
		}

		switch resp.StopReason {
		case anthropic.StopReasonToolUse:
			// All results go back in ONE user message.
			*messages = append(*messages, anthropic.NewUserMessage(results...))
		case anthropic.StopReasonMaxTokens:
			kit.Info("(stopped: hit max_tokens)")
			return nil
		case anthropic.StopReasonRefusal:
			kit.Info("(the model declined this request)")
			return nil
		default: // end_turn, stop_sequence
			return nil
		}
	}
}

func main() {
	kit.Info("s01 agent loop | model %s | type q to quit", cfg.Model)
	var history []anthropic.MessageParam
	kit.REPL("s01", func(ctx context.Context, line string) error {
		history = append(history, anthropic.NewUserMessage(anthropic.NewTextBlock(line)))
		return agentLoop(ctx, &history)
	})
}
