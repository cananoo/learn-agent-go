package main

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/cananoo/learn-agent-go/internal/fakeapi"
)

func TestSubagentHasFreshContextAndReturnsSummary(t *testing.T) {
	srv, client := fakeapi.New(t,
		// 0 parent: delegate
		fakeapi.Tools(fakeapi.ToolCall{ID: "p1", Name: "task", Input: map[string]any{"description": "probe", "prompt": "find the framework"}}),
		// 1 child: explore, and try to spawn a grandchild (must be refused)
		fakeapi.Tools(
			fakeapi.ToolCall{ID: "c1", Name: "bash", Input: map[string]any{"command": "echo exploring"}},
			fakeapi.ToolCall{ID: "c2", Name: "task", Input: map[string]any{"description": "x", "prompt": "y"}},
		),
		// 2 child: summary
		fakeapi.Text("framework: go test"),
		// 3 parent: done
		fakeapi.Text("done"),
	)
	cfg.Client, cfg.Workdir = client, t.TempDir()

	history := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("what framework?"))}
	if err := agentLoop(context.Background(), &history); err != nil {
		t.Fatal(err)
	}

	child := srv.Requests[1]
	if msgs := child["messages"].([]any); len(msgs) != 1 {
		t.Fatalf("subagent must start with a fresh context, got %d messages", len(msgs))
	}
	for _, tl := range child["tools"].([]any) {
		if name := tl.(map[string]any)["name"]; name == "task" || name == "todo" {
			t.Fatalf("subagent must not see the %s tool", name)
		}
	}
	if got := srv.ToolResults(2)["c2"]; !strings.Contains(got, "unknown tool") {
		t.Fatalf("recursive task call should be refused, got %q", got)
	}
	if got := srv.ToolResults(3)["p1"]; got != "framework: go test" {
		t.Fatalf("parent should receive only the summary, got %q", got)
	}
}
