# s05 · Skills

> Knowledge in two layers: a one-line catalog always, the full instructions only when needed.

[← s04](s04-subagents.md) · [Code](../../agents/s05_skills/main.go) · [Next: Compaction →](s06-compaction.md)

## The problem

You want the agent to follow your team's way of doing things: how to write tests, how to review, how to release. Putting all of it in the system prompt means paying for 10,000 tokens of instructions on every request, for knowledge used once a week. And the longer the system prompt, the less attention each part gets.

## The idea

Load knowledge the same way a person uses a bookshelf: read the spines, open the one book you need.

```
layer 1 · always in the system prompt (cheap):
    Available skills:
    - go-testing: Write or fix Go tests - table-driven tests, ...
    - code-review: Review a diff or a file for bugs ...

layer 2 · on demand, through a tool (full body):
    load_skill("go-testing")  →  <skill name="go-testing"> ...full SKILL.md... </skill>
```

## A skill is a folder

```
skills/
  go-testing/
    SKILL.md
  code-review/
    SKILL.md
```

```markdown
---
name: go-testing
description: Write or fix Go tests - table-driven tests, subtests, t.TempDir, httptest fakes
---

# Writing Go tests
1. Find the existing style first ...
```

The `description` is the most important line you write: it is all the model sees when deciding whether to load the skill. Describe *when* to use it, not just what it is.

## The code

Parsing needs no YAML library, just a cut on `---`:

```go
rest, ok := strings.CutPrefix(text, "---\n")
front, body, ok := strings.Cut(rest, "\n---")
```

`Catalog()` renders layer 1 into the system prompt; the `load_skill` tool returns layer 2. An unknown name returns the catalog in the error, so the model can recover from a typo.

## Why a tool result and not the system prompt?

Changing the system prompt mid-session would invalidate the prompt cache for everything after it. A tool result is appended at the end like any other message: cache-friendly, and visible exactly where the model needs it.

## Try it

```bash
go run ./agents/s05_skills
s05 >> review agents/s01_loop/main.go
```

It should call `load_skill("code-review")` first, then follow the checklist. Add your own skill folder and restart; no code change needed. Point `SKILLS_DIR` elsewhere to use a shared skill library.

## What changed from s04

- `skillSet` with `Catalog()` and `Load()`, about 80 lines.
- The catalog is appended to the system prompt; one new tool.
