package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/cananoo/learn-agent-go/internal/fakeapi"
)

func TestAutoCompactRestartsWithSummary(t *testing.T) {
	t.Setenv("COMPACT_THRESHOLD", "50") // fake responses report 110 tokens
	srv, client := fakeapi.New(t,
		fakeapi.Tools(fakeapi.ToolCall{ID: "t1", Name: "bash", Input: map[string]any{"command": "echo big-output"}}),
		fakeapi.Text("SUMMARY-XYZ: ran echo"), // the compaction call
		fakeapi.Text("done"),
	)
	cfg.Client, cfg.Workdir = client, t.TempDir()

	history := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("start"))}
	if err := agentLoop(context.Background(), &history); err != nil {
		t.Fatal(err)
	}

	summaryReq := srv.LastUserText(1)
	if !strings.Contains(summaryReq, "[tool result] big-output") || !strings.Contains(summaryReq, "[user] start") {
		t.Fatalf("summary request should contain the rendered transcript:\n%s", summaryReq)
	}
	if _, hasTools := srv.Requests[1]["tools"]; hasTools {
		t.Fatal("the summary call should not offer tools")
	}
	after := srv.Requests[2]["messages"].([]any)
	if len(after) != 1 || !strings.Contains(srv.LastUserText(2), "SUMMARY-XYZ") {
		t.Fatalf("history should restart from one summary message, got %d messages", len(after))
	}
	files, _ := filepath.Glob(filepath.Join(cfg.Workdir, ".transcripts", "*.json"))
	if len(files) != 1 {
		t.Fatalf("expected one saved transcript, got %v", files)
	}
	data, _ := os.ReadFile(files[0])
	if !strings.Contains(string(data), "big-output") {
		t.Fatal("transcript should hold the full history")
	}
}

func TestRenderTranscriptKeepsTheTail(t *testing.T) {
	msgs := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(strings.Repeat("old ", 100))),
		anthropic.NewUserMessage(anthropic.NewTextBlock("newest")),
	}
	out := renderTranscript(msgs, 50)
	if !strings.HasPrefix(out, "...(earlier part omitted)") || !strings.Contains(out, "newest") {
		t.Fatalf("got %q", out)
	}
}
