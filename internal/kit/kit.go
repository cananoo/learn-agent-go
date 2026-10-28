// Package kit holds the plumbing every lesson shares: reading .env, building
// the API client, colored output and the interactive prompt. None of it is
// about agents, which is exactly why it lives here and not in the lessons.
package kit

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// DefaultModel is used when MODEL_ID is not set.
const DefaultModel = "claude-opus-5-5"

// Config is what a lesson needs to talk to a model.
type Config struct {
	Client  anthropic.Client
	Model   string
	Workdir string
}

// Load reads .env (if any) and builds a client from the environment.
//
// The official SDK already understands ANTHROPIC_API_KEY,
// ANTHROPIC_AUTH_TOKEN and ANTHROPIC_BASE_URL, so pointing BASE_URL at any
// Anthropic-compatible endpoint (DeepSeek, Kimi, GLM, Qwen, Ollama...) just works.
func Load() Config {
	LoadDotEnv(".env")
	model := os.Getenv("MODEL_ID")
	if model == "" {
		model = DefaultModel
	}
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return Config{Client: anthropic.NewClient(), Model: model, Workdir: wd}
}

// LoadDotEnv sets KEY=VALUE pairs from path without overriding variables that
// are already set. A missing file is not an error.
func LoadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

// Platform describes the OS and shell so the system prompt can tell the model
// which commands will actually work.
func Platform() string {
	return fmt.Sprintf("%s/%s, shell: %s", runtime.GOOS, runtime.GOARCH, strings.Join(ShellCommand(""), " "))
}

// ShellCommand returns the argv used to run a shell command string. On
// Windows it prefers Git Bash when installed, because models write POSIX
// commands far more reliably than cmd.exe syntax.
func ShellCommand(command string) []string {
	if runtime.GOOS == "windows" {
		if bash := findGitBash(); bash != "" {
			return []string{bash, "-c", command}
		}
		return []string{"cmd", "/C", command}
	}
	return []string{"sh", "-c", command}
}

func findGitBash() string {
	for _, p := range []string{
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if gitDir := os.Getenv("GIT_INSTALL_ROOT"); gitDir != "" {
		p := filepath.Join(gitDir, "bin", "bash.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ANSI colors. Modern Windows terminals render them too.
const (
	gray   = "\033[90m"
	cyan   = "\033[36m"
	yellow = "\033[33m"
	red    = "\033[31m"
	reset  = "\033[0m"
)

// Text prints what the model says.
func Text(s string) { fmt.Println(s) }

// Tool prints a tool call and a short preview of its result.
func Tool(name, input, output string) {
	fmt.Printf("%s> %s%s %s\n", yellow, name, reset, Clip(input, 200))
	fmt.Printf("%s%s%s\n", gray, Clip(output, 400), reset)
}

// Info prints a dim status line.
func Info(format string, args ...any) {
	fmt.Printf(cyan+format+reset+"\n", args...)
}

// Error prints an error line.
func Error(format string, args ...any) {
	fmt.Printf(red+format+reset+"\n", args...)
}

// Clip shortens s to at most n runes for display.
func Clip(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + fmt.Sprintf(" ...(+%d chars)", len(r)-n)
}

// REPL reads lines from stdin and hands each one to handle until the user
// types q, exit, or sends EOF. The lesson name is shown in the prompt.
func REPL(name string, handle func(ctx context.Context, line string) error) {
	ctx := context.Background()
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1024*1024), 1024*1024)
	for {
		fmt.Printf("%s%s >> %s", cyan, name, reset)
		if !in.Scan() {
			fmt.Println()
			return
		}
		line := strings.TrimSpace(in.Text())
		switch strings.ToLower(line) {
		case "":
			continue
		case "q", "exit", "quit":
			return
		}
		if err := handle(ctx, line); err != nil {
			Error("error: %v", err)
		}
		fmt.Println()
	}
}
