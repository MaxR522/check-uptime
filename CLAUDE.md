# CLAUDE.md

## Working mode

Claude writes, edits, and refactors code to finish the project; the user reviews. (Learning phase logs: `docs/learning/`, don't read unless asked.)

- Complete, idiomatic Go. Stdlib first; dependencies only from "Decisions". Ask before any other new dependency, a spec change, or a non-additive schema change.
- One roadmap step at a time (`/roadmap-step`). Each step builds, passes `/verify`, and is a separate commit.
- Short explanations: what changed and why, a few lines.
- Commit only when asked. Conventional commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`).

## Project

`check-uptime` monitors the user's websites and endpoints against expected values (status, response time, body content), records incidents, sends notifications. Single binary: Go, SQLite, templ, htmx.

Docs, read only the sections a task needs: `docs/spec.md` (source of truth for features), `docs/database-schema.md`, `docs/code-architecture.md`.

Target layout (move code there as steps need it, not all at once; current code is in `cmd/check-uptime/main.go` and `internal/monitor/`):

```
cmd/check-uptime/main.go   config, slog, wiring, signals, graceful shutdown
internal/monitor/          domain types, assertions, checker, scheduler, incidents, Store interface
internal/store/sqlite/     Store implementation, embedded migrations
internal/notify/           Notifier interface, telegram, webhook
internal/ui/               handlers, routes; views/ (templ), static/ (htmx, CSS, embed.FS)
```

## Decisions

- Go 1.26: use `sync.WaitGroup.Go`, `new(expr)`, ServeMux method/wildcard patterns, `log/slog`, `errors.Join`.
- Dependencies: `modernc.org/sqlite` (no cgo), templ as a Go tool. htmx vendored. No ORM, query builder, migration library, or web framework. Details in `/sqlite-store` and `/ui-feature`.
- Config: flags with env fallback `CHECK_UPTIME_DB`, `CHECK_UPTIME_ADDR`, `CHECK_UPTIME_TELEGRAM_TOKEN`, `CHECK_UPTIME_TELEGRAM_CHAT_ID`, `CHECK_UPTIME_WEBHOOK_URL`.
- Durations: UI uses `1m`/`5m` strings; DB keeps `interval_check_min` and `timeout_sec`. Convert at the UI boundary.
- Incidents v1: open on first failure, resolve all open on first pass, at most one open per endpoint and reason (a new reason opens another incident).
- Notifications: Telegram first, webhook second.
- Open spec questions (retention, non-GET methods): ask when a step needs them.

## Conventions

- Errors: `fmt.Errorf("load websites: %w", err)`, lowercase, no trailing punctuation. Never drop silently; log intentional drops with `slog`.
- `context.Context` first parameter of every I/O function, respected for cancellation.
- Logging: `log/slog` only (replace `log`/`fmt.Println` when touching code). Keys: `website`, `endpoint_id`, `url`, `status`, `duration_ms`, `err`.
- Small interfaces defined at the consumer (`monitor.Store`, `monitor.Assertion`, `notify.Notifier`). Export domain types only when another package needs them. No stutter.
- Concurrency: every goroutine has an owner and a context to stop it. Results through channels; shared state behind a mutex. Must pass `go test -race`.
- Close resources: `resp.Body.Close()`, `rows.Close()`, check `rows.Err()`.
- Tests: table-driven, `t.Run`, `httptest.Server`, hand-written fakes, no mocking libraries.
- Make targets: `run`, `build`, `format`, `preview` (all use `./cmd/check-uptime`, binary `bin/check-uptime`).

## Roadmap status

Update when a step is done.

1. Domain types and assertions, tests: partial. Types exist (`website`, `endpoint`, `expectation`, `incident`, `checkResult`). `Assertion` interface, `status`/`max_response_time`/`body_contains`, tests missing. Status `>= 400` hard-coded in `checkEndpoint` should move to the `status` assertion.
2. Checker (`httptest` tests): partial. `checkEndpoint` works with context, body limit and UTF-8-safe error body; `httptest` tests exist; assertions not evaluated.
3. Scheduler: partial. Per-endpoint ticker goroutines in `checkAllEndpoints` (tested); SIGINT/SIGTERM cancel via `signal.NotifyContext` in `main`; no start/stop/restart of one endpoint schedule.
4. SQLite store with migrations: not started. `Store` interface in `internal/monitor/store.go`; `main` uses the in-memory `MemoryStore` (`memstore.go`, sample data) until the SQLite store replaces it.
5. Incident lifecycle: started. `recordIncident` opens an incident unless `Store.OpenIncidents` has one with the same reason (tested); resolve on pass not done.
6. Web UI (CRUD, then dashboard): not started.
7. Notifier, first channel: not started.
8. Later: consecutive failure threshold, TLS expiry, authentication.
