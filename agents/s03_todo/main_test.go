package main

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/cananoo/learn-agent-go/internal/fakeapi"
)

func echo(id string) fakeapi.ToolCall {
	return fakeapi.ToolCall{ID: id, Name: "bash", Input: map[string]any{"command": "echo " + id}}
}

func TestNagAfterRoundsWithoutTodo(t *testing.T) {
	srv, client := fakeapi.New(t,
		fakeapi.Tools(echo("a")), fakeapi.Tools(echo("b")), fakeapi.Tools(echo("c")),
		fakeapi.Text("done"),
	)
	cfg.Client, cfg.Workdir = client, t.TempDir()

	history := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("go"))}
	if err := agentLoop(context.Background(), &history); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(srv.LastUserText(2), "<reminder>") {
		t.Fatal("reminder came too early")
	}
	if !strings.Contains(srv.LastUserText(3), "<reminder>") {
		t.Fatal("expected a reminder after 3 rounds without todo")
	}
}

func TestTodoRejectsTwoInProgress(t *testing.T) {
	l := &todoList{}
	_, err := l.Update([]todoItem{{"1", "a", "in_progress"}, {"2", "b", "in_progress"}})
	if err == nil {
		t.Fatal("expected error for two in_progress items")
	}
	out, err := l.Update([]todoItem{{"1", "a", "completed"}, {"2", "b", "in_progress"}})
	if err != nil || !strings.Contains(out, "[x] 1: a") || !strings.Contains(out, "(1/2 completed)") {
		t.Fatalf("bad render %q, %v", out, err)
	}
}
