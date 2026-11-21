# Roadmap

One step = one session = one commit. Start each step in a fresh session (`/clear`), run `/roadmap-step`, and read only what the step lists under **Read**. Everything else is out of scope for that session.

Status values: `todo`, `doing`, `done`. Update the status line when a step is finished, and add a one-line note under **Notes** only when the next steps need to know something not visible in the code.

## How a step is sized

- Touches one area (one package plus `main.go` wiring at most) and loads at most one area skill.
- Reads at most about 5 source files plus the spec sections named. No step needs `docs/learning/`.
- Ends with `/verify` green and a reviewable diff (roughly under 600 changed lines including tests).
- Open questions are listed under **Ask**: ask them before writing code, and record the answer in `CLAUDE.md` "Decisions".

## Phase A: monitor core

### S1 Assertions

Status: `done`

- Goal: `Assertion` interface and the three v1 types, built from an `expectation` row.
- Read: spec "Assertions"; `internal/monitor/expectation.go`; skill `/add-assertion`.
- Touch: new `internal/monitor/assertion.go`, `assertion_test.go`.
- Build: `Assertion` interface (`Check(resp *http.Response, body []byte, d time.Duration) error`); `status` (exact list `200,201`, class `2xx`, regexp `^2\d\d$`), `max_response_time` (`250ms`), `body_contains`; `parseAssertion(expectation) (Assertion, error)` that validates the expected value (reused later by UI validation).
- Tests: table-driven per type, pass and fail cases, invalid expected values rejected by the parser.
- Done when: assertions exist and are tested; checker unchanged.

### S2 Checker evaluates assertions

Status: `done`

- Goal: a check fails on request error or any failing assertion, all reasons recorded.
- Read: spec "Assertions" last paragraph; `checker.go`, `checker_test.go`, `check_result.go`, `endpoint.go`, `assertion.go` (signatures only).
- Touch: `checker.go`, `check_result.go`, `checker_test.go`.
- Build: endpoint carries parsed assertions; `checkEndpoint` runs all of them and joins reasons (`errors.Join`); remove the hard-coded `>= 400`; `checkResult` gains `passed bool` and the list of failure reasons; error body kept on failure.
- Ask: endpoint with no `status` assertion: implicit `2xx`/`3xx` default, or pass on any status?
- Tests: `httptest.Server` cases for each assertion type failing, several failing together, network error, timeout.
- Notes: `checkResult.errMsg` is all reasons joined by `; `, so an incident reason is that joined string (S3: decide per-reason incidents). `passed` and `reasons` are not persisted yet (S6/S7). Invalid expectations skip the endpoint with a warning.

### S3 Incident resolve on pass

Status: `todo`

- Goal: full v1 incident lifecycle from "Decisions".
- Read: spec "Incidents"; `checker.go` (`handleResult`, `recordIncident`), `store.go`, `memstore.go`, `incident.go`, incident tests in `checker_test.go`.
- Touch: `store.go`, `memstore.go`, `checker.go`, new `incident_test.go` (move incident tests there).
- Build: `Store.ResolveIncidents(ctx, endpointID, at) ([]incident, error)`; on a passing result resolve every open incident; `recordIncident` returns the incidents it opened, resolve returns the ones it closed (these become notification events in S16).
- Tests: open on first failure, no duplicate same reason, new reason opens a second one, pass resolves all, pass with none open is a no-op.

### S4 Scheduler with per-endpoint control

Status: `todo`

- Goal: start, restart and stop one endpoint schedule at runtime.
- Read: spec "Check scheduling"; `checker.go` (`RunChecker`, `checkAllEndpoints`, `runCheckEndpoint`), scheduler tests in `checker_test.go`.
- Touch: new `internal/monitor/scheduler.go`, `scheduler_test.go`; `checker.go` uses it.
- Build: `Scheduler` owning a map `endpointID -> cancel` behind a mutex, `Start(ep)`, `Stop(id)`, `Restart(ep)` (stop then start), `Wait()`; goroutines through `sync.WaitGroup.Go`, results on one channel; first check runs immediately, then every interval.
- Tests: start/stop/restart with a short interval injected for tests, stop of unknown id is a no-op, shutdown via context leaves no goroutine running, all under `-race`.

## Phase B: persistence

### S5 Export domain types and config

Status: `todo`

- Goal: prepare `monitor` for a store in another package, add configuration.
- Read: `CLAUDE.md` "Decisions" (config); all `internal/monitor/*.go` type files; `main.go`.
- Touch: `internal/monitor/*.go` (rename to exported `Website`, `Endpoint`, `Expectation`, `Incident`, `CheckResult` with exported fields the store needs), `main.go`.
- Build: mechanical export (`gopls rename` or careful edits); `monitor.ErrNotFound`; flags with env fallback for `CHECK_UPTIME_DB` and `CHECK_UPTIME_ADDR` (notification flags come with their channel).
- Tests: existing tests still pass; small table test for flag/env precedence if config lives in a function.

### S6 SQLite foundation and migrations

Status: `todo`

- Goal: open the DB and apply embedded migrations.
- Read: skill `/sqlite-store`; `docs/database-schema.md`; spec "Concepts".
- Touch: new `internal/store/sqlite/{sqlite.go,migrate.go,migrate_test.go,migrations/0001_init.sql}`; `go.mod`; `docs/database-schema.md`.
- Build: `Open(ctx, path)` with pragmas, `user_version` migration runner, initial schema for all five tables. Add the columns the spec needs and the schema doc lacks: `websites.enabled`, `check_results.passed`. Unique open-incident index on `(endpoint_id, reason) WHERE resolved_at IS NULL` (the skill text says `endpoint_id` only; "Decisions" wins, fix the skill).
- Tests: fresh temp DB migrates, second `Open` is a no-op, foreign keys cascade, open-incident constraint.

### S7 SQLite Store for the checker

Status: `todo`

- Goal: `main` runs on SQLite instead of `MemoryStore`.
- Read: skill `/sqlite-store`; `internal/monitor/store.go`; `internal/store/sqlite/sqlite.go`; `main.go`.
- Touch: new `internal/store/sqlite/{monitor.go,monitor_test.go}`; `main.go`; delete `memstore.go` (tests keep hand-written fakes).
- Build: `GetWebsites` (websites, endpoints, expectations in three queries, grouped in Go), `OpenIncidents`, `StoreIncident`, `ResolveIncidents`, `StoreCheckResult`; compile-time check `var _ monitor.Store = (*Store)(nil)`.
- Tests: insert and read back, disabled website/endpoint filtering, resolve, check result round trip with nullable fields.
- Notes: until S11 the DB is filled by hand (`sqlite3`) or a test fixture.

### S8 SQLite CRUD for management

Status: `todo`

- Goal: write side the UI needs, defined as a consumer interface in `internal/ui` later.
- Read: skill `/sqlite-store`; `internal/store/sqlite/monitor.go` (helpers only).
- Touch: new `internal/store/sqlite/{manage.go,manage_test.go}`.
- Build: website create/get/list/update/set-enabled/delete; endpoint create/update/set-enabled/delete with its expectations in one transaction (replace-all on update); `ErrNotFound` mapping.
- Tests: CRUD round trips, cascade delete, transaction rollback on a bad expectation.

## Phase C: web UI

### S9 UI design

Status: `todo`

- Goal: agreed visual design for every screen before any templ code, so UI steps implement instead of invent.
- Read: spec "Website and endpoint management", "Dashboard", "Concepts"; skill `/ui-feature` sections "Components", "htmx patterns", "Validation" (constraints only). No Go code.
- Touch: a Claude Design artifact (Artifact `quickstart` with intent `design`); `docs/roadmap.md` Notes with the artifact link; no repo code.
- Build: screens for dashboard (website list, endpoint status badges up/down/unknown, open and recent incidents), website list with inline create/edit row, website page with endpoint list, endpoint form with assertion rows, endpoint check history; states for empty lists, validation errors next to fields, disabled items, delete confirm. Keep it buildable with plain semantic HTML and one hand-written `app.css`: design tokens (colors, spacing, type scale, light and dark) named so they map to CSS custom properties.
- Ask: design system to start from (default one offered by quickstart, or none), and visual direction (dense ops console vs airy).
- Done when: user approves the design; link and token list recorded under Notes; S10 to S15 read only the screen they implement.

### S10 UI skeleton and HTTP server

Status: `todo`

- Goal: server running next to the checker, empty layout page.
- Read: skill `/ui-feature`; S9 Notes (design tokens); `main.go`; templ and htmx docs via `ctx7` only for syntax in doubt.
- Touch: `go.mod` (templ tool), new `internal/ui/{handler.go,handler_test.go,views/layout.templ,static/htmx.min.js,static/app.css}`; `main.go`; `Makefile` (`generate` target, `build`/`run` depend on it); `/verify` skill if it needs the templ step.
- Build: `app.css` with the S9 tokens as CSS custom properties and base styles; `ui.NewHandler`, `GET /` with `Layout`, `/static/` from `embed.FS`; `http.Server` started in `run`, `Shutdown` on context cancel, checker and server each run in a goroutine owned by `run` (`sync.WaitGroup.Go`), first error cancels both.
- Tests: `httptest` request to `/` and to a static file.

### S11 Website CRUD

Status: `todo`

- Goal: manage websites from the UI.
- Read: S9 design, website list screen (Artifact `read`); skill `/ui-feature`; `internal/ui/handler.go`; website part of `internal/store/sqlite/manage.go` (signatures).
- Touch: new `internal/ui/{websites.go,websites_test.go,views/websites.templ}`.
- Build: list, create, edit, toggle enabled, delete with htmx fragments; URL validation with field errors and 422; consumer interface `websiteStore` in `ui`.
- Tests: handler tests with a fake store: create valid/invalid, edit, toggle, delete, htmx vs full-page responses.

### S12 Endpoint CRUD and live scheduling

Status: `todo`

- Goal: manage endpoints; schedules follow changes without restart.
- Read: S9 design, website page and endpoint form screen (Artifact `read`); skill `/ui-feature`; `internal/ui/websites.go` (pattern to copy); `internal/monitor/scheduler.go` (signatures); endpoint part of `manage.go` (signatures).
- Touch: new `internal/ui/{endpoints.go,endpoints_test.go,views/endpoints.templ}`; small `internal/monitor/service.go` (store write + scheduler start/restart/stop in one call); `main.go` wiring.
- Build: endpoint list inside the website page, create/edit/toggle/delete; `1m`/`5m` strings converted to `interval_check_min`/`timeout_sec` at the UI boundary; interval >= 1m, timeout < interval; website toggle also stops/starts its endpoints.
- Ask: allow methods other than `GET` now, or keep the form GET-only (spec open question)?
- Tests: handler tests with fakes, service test asserting the scheduler calls.

### S13 Assertion editing

Status: `todo`

- Goal: add and remove assertions in the endpoint form.
- Read: S9 design, assertion rows screen (Artifact `read`); skill `/ui-feature`; `internal/ui/endpoints.go`, `views/endpoints.templ`; `internal/monitor/assertion.go` (`parseAssertion` signature).
- Touch: `endpoints.go`, `views/endpoints.templ`, `endpoints_test.go`; maybe a new `views/assertions.templ`.
- Build: assertion rows (type select + value) with htmx add/remove row; server validation through `parseAssertion` so UI and checker share rules; save restarts the schedule.
- Tests: valid and invalid assertion submissions, error shown next to the row.

### S14 Dashboard status and incidents

Status: `todo`

- Goal: dashboard home with live status.
- Read: S9 design, dashboard screen (Artifact `read`); spec "Dashboard"; skill `/ui-feature`; `internal/ui/handler.go`; `internal/store/sqlite/monitor.go` (helpers).
- Touch: new `internal/store/sqlite/dashboard.go` + test; new `internal/ui/{dashboard.go,dashboard_test.go,views/dashboard.templ}`.
- Build: query for last result per endpoint (up/down/unknown, last time, duration, code), open incidents, recent resolved incidents; `GET /` renders it, `GET /dashboard/status` fragment polled `every 30s`.
- Tests: store query against temp DB (never-checked endpoint is unknown), handler test with fake.

### S15 Check history and retention

Status: `todo`

- Goal: per-endpoint history and bounded DB growth.
- Read: S9 design, history screen (Artifact `read`); spec "Dashboard", "Open questions"; `internal/store/sqlite/dashboard.go`; `internal/ui/dashboard.go`.
- Touch: `dashboard.go` (store + ui), new `views/history.templ`; new `internal/store/sqlite/prune.go` + test; `main.go` (prune goroutine).
- Ask: retention period for check results (7 or 30 days, flag?).
- Build: last 50 results per endpoint page or expandable fragment; periodic prune of old check results and old resolved incidents, owned goroutine stopped by context.
- Tests: history limit and order, prune boundary.

## Phase D: notifications

### S16 Notifier and incident events

Status: `todo`

- Goal: incident opened/resolved events reach notifiers without blocking checks.
- Read: spec "Notifications"; `internal/monitor/checker.go` (`handleResult`, incident functions).
- Touch: new `internal/notify/{notifier.go,dispatcher.go,dispatcher_test.go}`; `checker.go` emits events; `main.go`.
- Build: `notify.Notifier` interface (`Notify(ctx, Event) error`), `Event` (opened/resolved, website, endpoint, reason, times); dispatcher with buffered channel and one owned goroutine, per-send timeout, failures logged with `slog`, full buffer logged and dropped; checker depends on a small consumer-side interface.
- Tests: fake notifier receives events in order, failing notifier does not block, shutdown drains or stops cleanly under `-race`.

### S17 Telegram channel

Status: `todo`

- Goal: first real channel.
- Read: `internal/notify/notifier.go`; `main.go`; Telegram Bot API `sendMessage` via `ctx7`/web only if needed.
- Touch: new `internal/notify/{telegram.go,telegram_test.go}`; `main.go` flags `CHECK_UPTIME_TELEGRAM_TOKEN`, `CHECK_UPTIME_TELEGRAM_CHAT_ID`.
- Build: `POST /bot<token>/sendMessage` with context, base URL injectable for tests, token never logged; enabled only when both settings are present.
- Tests: `httptest.Server` checks path, payload and error status handling.

### S18 Webhook channel

Status: `todo`

- Goal: second channel.
- Read: `internal/notify/{notifier.go,telegram.go}`; `main.go`.
- Touch: new `internal/notify/{webhook.go,webhook_test.go}`; `main.go` flag `CHECK_UPTIME_WEBHOOK_URL`; fan-out when several channels are configured.
- Build: JSON `POST` of the event, non-2xx is an error.
- Tests: `httptest.Server` payload and status cases; fan-out test with two fakes.

## Phase E: hardening and later

### S19 End-to-end check and docs

Status: `todo`

- Goal: confidence that the wired binary works.
- Read: `main.go`; `README` if any; `Makefile`.
- Touch: new `cmd/check-uptime/main_test.go` (or `internal/app` if `run` needs extracting); `README.md`; `CLAUDE.md`.
- Build: test that starts `run` with a temp DB, a target `httptest.Server` and a fake notifier, creates a website through the UI handler, waits for a failing then passing check, asserts incident open/resolve and two events. README: build, run, flags.

### Later (one session each, plan when reached)

- S20 Consecutive failure threshold before opening an incident (spec open question; changes "Decisions").
- S21 TLS certificate expiry check as a new assertion type (`/add-assertion`).
- S22 Authentication (single user, out of v1 scope).
