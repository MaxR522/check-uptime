---
name: ui-feature
description: Conventions for check-uptime's web UI built with templ components, htmx partial updates, and stdlib http.ServeMux handlers. Use when adding or changing pages, forms, fragments, routes, or the dashboard.
---

# UI feature (templ + htmx)

Before writing templ or htmx syntax you are unsure of, fetch current docs with `ctx7` (`/a-h/templ`, and htmx).

## Layout

- Handlers and routes in `internal/ui`. Components in `internal/ui/views/*.templ`. Static files (`htmx.min.js`, `app.css`) in `internal/ui/static`, served with `embed.FS` and `http.FileServerFS` under `/static/`.
- `ui.NewHandler(svc, logger) http.Handler` builds a `*http.ServeMux` with method patterns: `GET /`, `GET /websites/new`, `POST /websites`, `GET /websites/{id}/edit`, `PUT /websites/{id}`, `DELETE /websites/{id}`, and the same shape for endpoints under `/websites/{id}/endpoints/...`. Read ids with `r.PathValue("id")` and `strconv.Atoi`.
- Handlers depend on a small interface defined in `ui` listing only the service methods they call, so tests use a fake.
- templ is a Go tool (`go get -tool github.com/a-h/templ/cmd/templ@latest`). After every `.templ` change: `go tool templ generate`. Commit the generated `*_templ.go`.
- Handlers validate input server side; on failure re-render the form fragment with field errors, status 422. htmx requests get only the swapped fragment.

## Components

- One `Layout(title string)` component with `{ children... }`, loading htmx and CSS. Full pages wrap content in it; fragments do not.
- Split components so each htmx target is its own component (`WebsiteRow`, `WebsiteForm`, `EndpointList`, `StatusBadge`). The handler renders the same component for the full page and for the fragment.
- Forms take a view model with the submitted values and a `map[string]string` of field errors; show the error next to its field.
- Plain semantic HTML and a small hand-written CSS file. No JS framework, no Tailwind build step.

## htmx patterns

- Create: `hx-post`, `hx-target` the list, `hx-swap="beforeend"`; on success return the new row and reset the form.
- Edit: row swaps to form with `hx-get .../edit`, `hx-put` returns the updated row, `hx-swap="outerHTML"`.
- Delete: `hx-delete`, `hx-confirm`, return empty body with 200 and `hx-swap="outerHTML"` to remove the row.
- Toggle enabled: `hx-patch` returning the updated row.
- Dashboard refresh: `hx-get="/dashboard/status" hx-trigger="every 30s" hx-swap="outerHTML"` on the status block only.
- Validation failure: status 422 and the form fragment with errors. htmx does not swap 4xx by default; either configure `htmx.config.responseHandling` for 422 or use the `HX-Retarget`/`HX-Reswap` response headers. Check the htmx docs for the version vendored.
- Detect htmx with `r.Header.Get("HX-Request") == "true"`; non-htmx requests get the full page or a redirect (303 after POST).

## Validation (server side, from the spec)

- Base URL: `url.Parse`, scheme `http` or `https`, non-empty host.
- Path starts with `/`.
- Interval: `time.ParseDuration`, at least 1 minute. Timeout: parsed, positive, lower than the interval.
- Status assertion: exact list, class (`2xx`), or a regexp that compiles. Durations parse. `body_contains` non-empty.
- Validation lives in `monitor` (one function per input type) so the UI and tests share it; the handler maps errors to fields.

## Scheduler integration

Create, edit, disable, delete of an endpoint must start, restart, or stop its schedule without restarting the app: the handler calls the service, the service calls the scheduler. The UI never touches goroutines directly.

## Tests

`httptest.NewRecorder` with the real mux and a fake service: status codes, that fragments are returned for `HX-Request`, that 422 carries field errors, that path values are parsed. Assert on key substrings of the HTML, not full documents.
