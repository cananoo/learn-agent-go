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

func TestParallelFileToolsInOneTurn(t *testing.T) {
	srv, client := fakeapi.New(t,
		fakeapi.Tools(
			fakeapi.ToolCall{ID: "w", Name: "write_file", Input: map[string]any{"path": "a/hello.txt", "content": "hello world"}},
			fakeapi.ToolCall{ID: "x", Name: "read_file", Input: map[string]any{"path": "../outside.txt"}},
		),
		fakeapi.Tools(fakeapi.ToolCall{ID: "e", Name: "edit_file", Input: map[string]any{"path": "a/hello.txt", "old_text": "world", "new_text": "gopher"}}),
		fakeapi.Text("done"),
	)
	cfg.Client, cfg.Workdir = client, t.TempDir()

	history := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("go"))}
	if err := agentLoop(context.Background(), &history); err != nil {
		t.Fatal(err)
	}
	first := srv.ToolResults(1)
	if len(first) != 2 {
		t.Fatalf("both parallel results must come back in one message, got %v", first)
	}
	if !strings.Contains(first["x"], "escapes workspace") {
		t.Fatalf("path traversal not blocked: %q", first["x"])
	}
	data, _ := os.ReadFile(filepath.Join(cfg.Workdir, "a", "hello.txt"))
	if string(data) != "hello gopher" {
		t.Fatalf("edit not applied, file is %q", data)
	}
}

func TestEditFileRequiresUniqueMatch(t *testing.T) {
	cfg.Workdir = t.TempDir()
	os.WriteFile(filepath.Join(cfg.Workdir, "f.go"), []byte("x x"), 0o644)
	if _, err := editFile("f.go", "x", "y"); err == nil || !strings.Contains(err.Error(), "2 times") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
	if _, err := editFile("f.go", "zzz", "y"); err == nil {
		t.Fatal("expected not-found error")
	}
}
