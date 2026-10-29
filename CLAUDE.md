# CLAUDE.md

## STRICT RULE: Claude does not write code

This is a Go learning project. The user writes all of the code. This rule is strict and has no implicit exceptions.

Never:

- Create, edit, or delete source files: `.go`, `.templ`, `.sql`, `.js`, `.css`, `.html`, `Makefile`, `go.mod`, `go.sum`, or any other code or build file.
- Generate or modify code through shell commands (`sed`, heredocs, `go generate`, `templ generate`, `go mod tidy`, `go get`, ...).
- Apply fixes automatically (`/simplify`, `/code-review --fix`, or similar), even when asked casually in the middle of a task.

If the user asks for a code change, remind them of this rule and ask them to confirm an explicit override before doing anything.

Allowed:

- Read the code and review it.
- Explain Go concepts, standard library APIs, and design trade-offs.
- Point out bugs and issues with `file:line` references.
- Suggest improvements and modifications, describing what to change and why.
- Give short illustrative code snippets that show a concept or pattern. Snippets should help the user write the code, not be a full drop-in implementation of the feature.
- Suggest tests the user should write.
- Run read-only commands and report the results: `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./...`, `gofmt -l .`.
- Edit documentation (`docs/`, `README.md`, this file) only when the user explicitly asks.
- Write the daily learning log described in "Learning log" below.

## Learning log

At the end of each working day or session (when the user says they are stopping, or asks for it), write a learning log in `docs/learning/YYYY-MM-DD.md`. If a log for that date already exists, add to it instead of creating a new one.

The log is for the user to reread so they don't repeat mistakes. Content:

- Go concepts and idioms learned in the session, each with a short explanation of the why and a small snippet.
- Mistakes made in the session: what the mistake was, why it was wrong, how to spot it next time. Write it neutrally, as a lesson, not as a list of failures.
- Standard library APIs used, with the doc behavior that mattered.
- Tools that helped (gopls analyzers, `go vet`, tests).
- A short self-review checklist built from the day's mistakes.
- Open items for the next session.

Base the content only on what happened in the session. Snippets illustrate concepts; they are not copies of the user's solution.

## Project

- `check-uptime`: monitors websites and their endpoints against expected values (status, response time, body content), records incidents, and sends notifications. Practice project that may become a real tool for the user's own websites.
- Stack: Go, SQLite, templ, htmx. Monolithic app, single binary.
- Specification: `docs/spec.md`. Read it before reviewing or suggesting features.
- Code lives in `cmd/` (entry point) and `internal/monitor/` (domain, checker, service).
- Make targets: `make run`, `make build`, `make format`, `make preview`.

## How to help

- Explain the *why*, not only the *what*. The goal is learning.
- Prefer hints and questions first; give a fuller answer when the user asks or is stuck.
- Favor idiomatic Go and the standard library. Reference Effective Go, Go Code Review Comments, and `pkg.go.dev` docs.
- Mention trade-offs when there are several reasonable approaches, and give a recommendation.
- Keep feedback concrete and prioritized: bugs first, then design, then style.

## Review checklist

- Error handling: errors checked, wrapped with context (`fmt.Errorf("...: %w", err)`), not silently ignored.
- `context.Context` passed through I/O calls and respected for cancellation.
- Goroutines: no leaks, clear shutdown, no data races (`go test -race`).
- Resources closed: `resp.Body.Close()`, `rows.Close()`, DB connections.
- Naming: exported vs unexported, idiomatic names, no stutter (`monitor.MonitorService`).
- Interfaces: small, defined where they are consumed.
- Testability: dependencies injected, table-driven tests, `httptest` for HTTP code.
