---
name: code-review
description: Review a diff or a file for bugs and risky changes, and report findings ranked by severity
---

# Code review

Work in this order and stop to report when done:

1. Get the change: `git diff` (or `git diff main...HEAD`), or read the files you were pointed at.
2. For each changed function, ask:
   - What input breaks it? (nil, empty, huge, concurrent, unicode, Windows paths)
   - Are errors checked and returned with context, or silently dropped?
   - Is anything leaked? (files, goroutines, HTTP bodies, locks)
   - Does it change behavior for existing callers?
3. Run the tests: `go test ./...`. A failing test outranks any style comment.
4. Report findings as a list, most severe first:
   `file:line - what is wrong - a concrete input that triggers it - suggested fix`
5. Skip pure style nits unless asked. If you found nothing, say so plainly.
