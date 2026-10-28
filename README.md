<div align="center">

# learn-agent-go

**Build a Claude Code–style coding agent in Go, from scratch, one idea at a time.**

[![ci](https://github.com/cananoo/learn-agent-go/actions/workflows/ci.yml/badge.svg)](https://github.com/cananoo/learn-agent-go/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.24+-00ADD8?logo=go)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

English | [简体中文](README_zh.md)

</div>

An agent is a loop. Everything else — tools, plans, subagents, skills, memory — is something you add *around* that loop without changing it.

This repo builds a working coding agent in **six small steps**. Each step is **one runnable Go file** that adds exactly one idea to the previous one, so you can `diff` your way through the whole design. No framework, no magic: just the official Anthropic Go SDK and the standard library.

```go
for {
    resp := model(messages, tools)
    messages = append(messages, resp)
    if resp.StopReason != "tool_use" {
        return                          // the model is done
    }
    messages = append(messages, runTools(resp))  // feed results back
}
```

## The lessons

| # | Lesson | One idea | Lines |
|---|--------|----------|------:|
| [s01](agents/s01_loop/main.go) | **The agent loop** | One loop + one `bash` tool is already an agent | 141 |
| [s02](agents/s02_tools/main.go) | **Tools** | New capability = schema + handler in a dispatch map; the loop never changes | 270 |
| [s03](agents/s03_todo/main.go) | **Planning** | A `todo` tool that does nothing but keep the plan visible — plus a nag when it's ignored | 358 |
| [s04](agents/s04_subagent/main.go) | **Subagents** | Same loop, empty `messages`: explore 30 files, return one paragraph | 428 |
| [s05](agents/s05_skills/main.go) | **Skills** | Two-layer knowledge: a one-line catalog always, the full `SKILL.md` on demand | 524 |
| [s06](agents/s06_compact/main.go) | **Compaction** | When context fills up: save transcript, summarize, restart append-only | 644 |

Each lesson has a short write-up: [English](docs/en) · [中文](docs/zh).

## Quick start

```bash
git clone https://github.com/cananoo/learn-agent-go
cd learn-agent-go
cp .env.example .env        # put your key in .env
go run ./agents/s01_loop
```

Then try:

```
s01 >> create hello.go that prints the first 10 primes, then run it
s03 >> add a --verbose flag to hello.go and test both modes        (watch the plan)
s04 >> which packages in this repo have no tests? use a subagent  (watch the context stay small)
s05 >> review agents/s01_loop/main.go                              (watch it load the code-review skill)
```

> ⚠️ The agent runs real shell commands in the current directory. Use a scratch directory or a container. The command blocklist is a seatbelt, not a sandbox.

## Use any model with an Anthropic-compatible API

The official SDK reads `ANTHROPIC_API_KEY` / `ANTHROPIC_AUTH_TOKEN` and `ANTHROPIC_BASE_URL`, so every provider that speaks the Messages API works with zero code changes:

| Provider | `ANTHROPIC_BASE_URL` | `MODEL_ID` |
|---|---|---|
| Anthropic (default) | *(unset)* | `claude-opus-5-5`, `claude-sonnet-5-5`, `claude-haiku-5-5` |
| DeepSeek | `https://api.deepseek.com/anthropic` | see provider docs |
| Moonshot Kimi | `https://api.moonshot.cn/anthropic` | see provider docs |
| Zhipu GLM | `https://open.bigmodel.cn/api/anthropic` | see provider docs |
| Ollama (local) | `http://localhost:11434` | any tool-calling model you pulled |

Endpoints come from each provider's public docs; tool-calling quality varies a lot between models. If a provider works (or doesn't) for you, please open an issue so the table stays honest.

## Why build it yourself?

Because once you've written the loop, every agent product stops being magic. You'll recognize s03 in every "planning" feature, s04 in every "research mode", s05 in every "rules / skills / memory bank", and s06 in every "context was summarized" banner. You'll also know which knobs actually matter when your own agent misbehaves.

**Why Go?** The message structures are explicit types, not dicts; the whole agent compiles to a single binary; and goroutines make the next steps (parallel tools, background subagents) natural.

## How it's tested

Every lesson's loop is tested **offline** against a scripted fake of the Messages API ([internal/fakeapi](internal/fakeapi/fakeapi.go)) — no key needed:

```bash
go test ./...
```

The tests check what matters for an agent: parallel tool results go back in one message, subagents start with an empty context and can't spawn more subagents, path traversal is blocked, compaction restarts from a single summary message, and so on.

## Layout

```
agents/
  s01_loop/      the loop + bash
  s02_tools/     + read/write/edit files, dispatch map
  s03_todo/      + todo planning, reminder
  s04_subagent/  + task tool, fresh-context child loop
  s05_skills/    + skills catalog + load_skill
  s06_compact/   + auto/manual compaction (the complete agent)
skills/          example skills (go-testing, code-review)
internal/kit     env, client, printing, REPL — no agent logic
internal/fakeapi scripted Messages API for tests
docs/en, docs/zh lesson write-ups
```

## Roadmap

- [ ] s07 permissions — ask before risky tools, allow-lists
- [ ] s08 streaming output
- [ ] s09 MCP — tools from external servers
- [ ] s10 parallel subagents with goroutines

Ideas and PRs welcome. If this helped you understand agents, a ⭐ helps other people find it.

## Credits

Inspired by the Python project [shareAI-lab/learn-claude-code](https://github.com/shareAI-lab/learn-claude-code). This is an independent Go take with its own lesson design, and is not affiliated with Anthropic.

## License

[MIT](LICENSE)
