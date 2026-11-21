---
name: add-assertion
description: Add a new check-uptime assertion type (expected-value rule on an endpoint response) behind the Assertion interface, with parsing, validation, and table-driven tests. Use when adding status, max_response_time, body_contains, body_regexp, header_equals, json_path, or any custom check.
---

# Add an assertion type

The spec requires that a new assertion type never needs a change in the checker.

## Shape

```go
type Assertion interface {
	Check(resp *http.Response, body []byte, duration time.Duration) error
}
```

- Each type is a small struct in `internal/monitor/assertion_<name>.go` holding its parsed config (compiled regexp, parsed duration, status set), not the raw string.
- `Check` returns `nil` on pass, or an error whose message is a readable reason recorded on the check result and incident (`status 503 not in 2xx`, `response took 812ms, max 500ms`, `body does not contain "ok"`).
- A registry maps the `criteria` string stored in `expectations.criteria` to a parse function: `map[string]func(expected string) (Assertion, error)`. Parsing is also the validation the UI uses, so invalid config is rejected when saved, not when checked.
- The checker evaluates every assertion and joins failures (`errors.Join`), so all reasons are recorded, not only the first.

## Steps

1. Add the struct, its `Check`, and its parse function; register it under its criteria name.
2. Table-driven tests in `assertion_<name>_test.go`: parse cases (valid, invalid config) and check cases (pass, fail with the expected reason). Build `*http.Response` values directly; no server needed.
3. If the UI has an assertion form, add the new type to its select and any help text.
4. Add the type to the "Assertions" section of `docs/spec.md` if it is new there.
5. Run `/verify`.
