# Check uptime of a website

## Goal

Check the health of websites against expected values, record incidents when they fail, and notify the owner. The project is a monolithic Go application: Go backend, SQLite database, server-rendered UI with templ and htmx, shipped as a single binary.

## Concepts

- **Website**: a site to monitor. Fields: name, base URL (example: `https://example.com`), enabled flag.
- **Endpoint (path)**: one path to test inside a website (example: `/health`, `/`). Fields: path, HTTP method (default `GET`), check interval, request timeout, enabled flag, list of assertions.
- **Assertion**: one expected-value rule an endpoint response must satisfy (status, response time, body content, ...).
- **Check result**: outcome of one check run. Fields: endpoint, timestamp, status code, response duration, error (network error, timeout, ...), pass/fail, failed assertion reasons.
- **Incident**: a period during which an endpoint is failing. Opened on failure, resolved when the endpoint passes again.

## Features

### Website and endpoint management (UI)

- The user creates, lists, edits, enables/disables and deletes websites through the web UI.
- Inside a website, the user creates, edits, enables/disables and deletes endpoints and their assertions.
- UI built with templ components and htmx partial updates (forms submit and swap only the affected fragment, no full page reload).
- Inputs are validated server side (valid URL, path starts with `/`, valid durations, valid regexp, ...) and errors are shown next to the field.

### Check scheduling

- Each endpoint has its own check interval, written as a duration (`1m`, `5m`, `1h`). Example: `/health` every 5 minutes, `/` every 30 minutes.
- Minimum interval: 1 minute, to avoid hammering the monitored site.
- Each endpoint has a request timeout (default 10s). The timeout must be lower than the interval.
- Endpoints are checked concurrently, each on its own schedule.
- When an endpoint is created, edited, disabled or deleted in the UI, its schedule is started, restarted or stopped without restarting the app.
- On shutdown (SIGINT/SIGTERM) running checks are cancelled through `context` and the app exits cleanly.

### Assertions (expected values)

Assertions are extensible: each type implements a common interface, so adding a new custom check does not require changing the checker. Example idea:

```go
type Assertion interface {
	Check(resp *http.Response, body []byte, duration time.Duration) error
}
```

Built-in types for v1:

- `status`: expected status code. Supports an exact list (`200`, `200,201`), a class (`2xx`), or a regexp (`^2\d\d$`).
- `max_response_time`: response must arrive in less than the given duration (example: `250ms`, `500ms`).
- `body_contains`: response body must contain a given string.

Possible later types: `body_regexp`, `header_equals`, `json_path` (value at a JSON path equals something).

A check fails if the request errors (DNS, connection, timeout) or if any assertion fails. All failed assertion reasons are recorded, not only the first one.

### Incidents

- When a check fails and the endpoint has no open incident, a new incident is opened with the failure reason and start time.
- While the endpoint keeps failing, no new incident is created (at most one open incident per endpoint).
- When a check passes and an incident is open, the incident is resolved (`resolved_at` set).
- Every check result is logged.

### Notifications

- A notification is sent when an incident is opened and when it is resolved.
- Notification channels implement a common `Notifier` interface, so new channels can be added (webhook, Discord, Slack, email, ...).
- A failing notification is logged and does not block or crash the checker.

### Dashboard

- List of websites with the current status of each endpoint: up, down, or unknown (never checked yet).
- For each endpoint: last check time, last response time, last status code.
- List of open incidents and recent resolved incidents.
- Recent check history per endpoint (example: last 50 results).
- The dashboard refreshes periodically with htmx polling.

## Non-functional

- Configuration through flags and environment variables: database path, HTTP listen address, notification settings.
- Structured logging with `log/slog`.
- Tests: table-driven unit tests for assertions, `httptest.Server` for the checker, tests for incident open/resolve logic.
- Code passes `go vet` and `go test -race`.
- Single binary, no external service required except notification targets.

## Non-goals (v1)

- Authentication and multiple users.
- Checks from multiple regions / distributed checkers.
- TLS certificate expiry checks.
- SMS or phone notifications.

## Roadmap

Suggested order, each step usable and testable on its own:

1. Domain types and assertions, with unit tests.
2. Checker: perform an HTTP request, measure duration, evaluate assertions (tested with `httptest`).
3. Scheduler: goroutines, tickers, `context` cancellation, graceful shutdown.
4. SQLite store implementing the `Store` interface, with migrations.
5. Incident lifecycle (open / resolve).
6. Web UI with templ and htmx: website and endpoint CRUD, then dashboard.
7. Notifier with a first channel.
8. Later: retry / consecutive failure threshold before opening an incident, TLS certificate expiry, authentication.

## Open questions

- Which notification channel first (webhook, Discord, Slack, email)?
- Retention period for check results (7 days, 30 days)?
- Open an incident on the first failure, or only after N consecutive failures to avoid flapping?
- Support HTTP methods other than `GET`, custom headers, or a request body?
