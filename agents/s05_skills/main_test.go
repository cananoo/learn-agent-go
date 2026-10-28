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

func TestRepoSkillsParse(t *testing.T) {
	set := loadSkills(filepath.Join("..", "..", "skills"))
	if len(set) < 2 {
		t.Fatalf("expected the example skills to load, got %d", len(set))
	}
	if !strings.Contains(set.Catalog(), "- go-testing: ") {
		t.Fatalf("catalog missing go-testing:\n%s", set.Catalog())
	}
}

func TestParseSkillRejectsMissingFields(t *testing.T) {
	if _, err := parseSkill("---\nname: x\n---\nbody"); err == nil {
		t.Fatal("expected error without description")
	}
	s, err := parseSkill("---\r\nname: x\r\ndescription: \"does x\"\r\n---\r\nbody line\r\n")
	if err != nil || s.Description != "does x" || s.Body != "body line" {
		t.Fatalf("CRLF skill parsed wrong: %+v %v", s, err)
	}
}

func TestLoadSkillTool(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "deploy"), 0o755)
	os.WriteFile(filepath.Join(dir, "deploy", "SKILL.md"), []byte("---\nname: deploy\ndescription: ship it\n---\nRun make deploy."), 0o644)
	skills = loadSkills(dir)

	srv, client := fakeapi.New(t,
		fakeapi.Tools(
			fakeapi.ToolCall{ID: "k1", Name: "load_skill", Input: map[string]any{"name": "deploy"}},
			fakeapi.ToolCall{ID: "k2", Name: "load_skill", Input: map[string]any{"name": "nope"}},
		),
		fakeapi.Text("done"),
	)
	cfg.Client, cfg.Workdir = client, t.TempDir()
	history := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("deploy"))}
	if err := agentLoop(context.Background(), &history); err != nil {
		t.Fatal(err)
	}
	res := srv.ToolResults(1)
	if !strings.Contains(res["k1"], "Run make deploy.") {
		t.Fatalf("skill body not returned: %q", res["k1"])
	}
	if !strings.Contains(res["k2"], "available:") {
		t.Fatalf("unknown skill should list available ones: %q", res["k2"])
	}
}
