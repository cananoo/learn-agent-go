package main

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/cananoo/learn-agent-go/internal/fakeapi"
)

func TestAgentLoopRunsToolsUntilDone(t *testing.T) {
	srv, client := fakeapi.New(t,
		fakeapi.Tools(fakeapi.ToolCall{ID: "t1", Name: "bash", Input: map[string]any{"command": "echo hello-agent"}}),
		fakeapi.Text("done"),
	)
	cfg.Client, cfg.Workdir = client, t.TempDir()

	history := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("say hello"))}
	if err := agentLoop(context.Background(), &history); err != nil {
		t.Fatal(err)
	}
	if srv.Calls() != 2 {
		t.Fatalf("want 2 model calls, got %d", srv.Calls())
	}
	if got := srv.ToolResults(1)["t1"]; !strings.Contains(got, "hello-agent") {
		t.Fatalf("tool result not sent back, got %q", got)
	}
	// user, assistant(tool_use), user(tool_result), assistant(text)
	if len(history) != 4 {
		t.Fatalf("want 4 messages in history, got %d", len(history))
	}
}

func TestRunBashBlocksDangerousCommands(t *testing.T) {
	if got := runBash("sudo rm -rf /"); !strings.Contains(got, "blocked") {
		t.Fatalf("expected block, got %q", got)
	}
}
