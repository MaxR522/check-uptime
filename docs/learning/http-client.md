# Making HTTP requests in Go (`net/http`)

Lesson from 2026-10-29. Covers the simple way (`http.Get`) and the context-aware way (`http.NewRequestWithContext` + `client.Do`).

## 1. The main types (client side)

| Type | Role |
|---|---|
| `http.Client` | Sends requests. Holds the timeout, redirect policy and Transport. Safe for concurrent use: create one, reuse it. |
| `http.Request` | What you send: method, URL, headers, body, context. |
| `http.Response` | What you get back: `StatusCode`, `Header`, `Body` (an `io.ReadCloser`). |
| `http.Transport` | Low level: TCP connections, TLS, connection pooling. Rarely touched directly. |

The flow is always: build a Request, call `client.Do(req)`, get a Response, read the Body, close the Body.

## 2. The simple way: `http.Get`

```go
resp, err := http.Get("https://example.com")
if err != nil {
    return fmt.Errorf("get example: %w", err) // network error, DNS, TLS...
}
defer resp.Body.Close()

body, err := io.ReadAll(resp.Body)
if err != nil {
    return fmt.Errorf("read body: %w", err)
}
fmt.Println(resp.StatusCode, len(body))
```

Rules that matter:

- A non-nil `err` means a transport-level failure (DNS, connection refused, timeout). A 404 or 500 is **not** an error: `err == nil` and you must check `resp.StatusCode` yourself.
- Always `defer resp.Body.Close()`, and only after checking `err` (on error, `resp` is nil). Forgetting it leaks the connection and prevents pool reuse.
- If the body is not read to EOF before closing, the Transport may not reuse the connection. Reading it (even through an `io.LimitReader`) is good.

Problems with `http.Get`:

- It uses `http.DefaultClient`, which has **no timeout**. A slow server can block the goroutine forever. Fatal for an uptime checker.
- It cannot be cancelled.
- You cannot set headers.

`http.Post` and `http.Head` have the same problems.

## 3. Middle step: your own Client with a Timeout

```go
client := &http.Client{Timeout: 10 * time.Second}
resp, err := client.Get("https://example.com")
```

`Client.Timeout` covers the whole exchange: connecting, sending, receiving headers, and reading the body. Better, but still no way to cancel from the outside (for example on app shutdown).

### Client vs Transport: where the connection pool lives

`http.Get` does not create anything per call: it calls `DefaultClient.Get`, and `DefaultClient` is a package-level variable (`var DefaultClient = &Client{}`) shared by the whole program. So "avoid re-initializing the client" is not the reason to define your own.

The connection pool (open TCP connections, TLS sessions) lives in the **Transport**, not the Client. A Client is a small struct of settings and is cheap to create. From the docs: if `Client.Transport` is nil, `DefaultTransport` is used.

| Code | Pool |
|---|---|
| `http.Get(...)` | `DefaultTransport`, shared |
| `&http.Client{Timeout: 5 * time.Second}` created on every call | `DefaultTransport`, still shared |
| `&http.Client{Transport: &http.Transport{}}` created on every call | New pool each time: new TCP + TLS handshake per request, idle connections pile up |

The real trap is a new **Transport** per request.

Reasons to define your own Client:

1. **Timeout**: `DefaultClient` has none. The main reason.
2. **`CheckRedirect`**: by default the client follows up to 10 redirects. An uptime checker may want to see the 301 itself; returning `http.ErrUseLastResponse` stops at the first response.
3. **Custom Transport** when needed: `MaxIdleConnsPerHost`, TLS config, proxy.
4. **Isolation**: changing `http.DefaultClient` or `DefaultTransport` affects the whole program, including libraries. Your own client affects only your code.
5. **Testability**: inject the client (as `NewChecker` does); tests can pass `httptest.NewServer(...).Client()`.

Reuse one Client per configuration (it is safe for concurrent use) so settings stay consistent and a future custom Transport is not recreated per request.

## 4. The context way: `NewRequestWithContext` + `client.Do`

```go
ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
defer cancel() // always release the context resources

req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
if err != nil {
    return fmt.Errorf("build request: %w", err) // bad URL or method
}
req.Header.Set("User-Agent", "check-uptime/0.1")

resp, err := client.Do(req)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        // timed out because of the ctx
    }
    if errors.Is(err, context.Canceled) {
        // parent was cancelled (shutdown, Ctrl+C)
    }
    return fmt.Errorf("do request: %w", err)
}
defer resp.Body.Close()
```

From `go doc net/http.NewRequestWithContext`:

> For an outgoing client request, the context controls the entire lifetime of a request and its response: obtaining a connection, sending the request, and reading the response headers and body.

So cancelling the context stops the request at any stage, even in the middle of reading the body.

Why this is the idiomatic way:

- **Cancellation propagates.** `main` cancels the root context on SIGINT, and every in-flight HTTP call stops. `Client.Timeout` alone cannot do that.
- **Per-call deadlines.** Each endpoint can get its own timeout (useful for a response-time expectation) by wrapping the call in `context.WithTimeout`.
- **Full control** over method, headers and body.
- `client.Do` returns a `*url.Error` that wraps the cause, so `errors.Is` still finds `context.DeadlineExceeded`.

### `Client.Timeout` vs context timeout

Both apply; the shorter one wins. Common pattern: keep `Client.Timeout` as a hard safety net, and use the context for cancellation and per-call deadlines.

### Sending a body (POST)

Pass an `io.Reader` as the last argument and set the content type:

```go
req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(`{"a":1}`))
req.Header.Set("Content-Type", "application/json")
```

## 5. Checklist

- [ ] Check `err` first, then `defer resp.Body.Close()`.
- [ ] Never use `http.Get` / `http.DefaultClient` in real code: no timeout.
- [ ] Create one `http.Client` per configuration and reuse it. Never create a new `http.Transport` per request: that loses the connection pool.
- [ ] A status code ≥ 400 is not a Go error. Check `StatusCode` explicitly.
- [ ] Wrap the body in `io.LimitReader` so an untrusted server cannot send gigabytes.
- [ ] `defer cancel()` right after `context.WithTimeout` / `WithCancel` (`go vet` flags a missing cancel).
- [ ] Use the `http.MethodGet` constants instead of the string `"GET"`.

## 6. Exercises

1. Write a small `main` that calls `http.Get("https://httpbin.org/delay/15")`. Watch it hang. Then use a Client with `Timeout: 3 * time.Second` and print the error.
2. Same URL with `NewRequestWithContext` and `context.WithTimeout(..., 2*time.Second)`. Check `errors.Is(err, context.DeadlineExceeded)`.
3. Set both a Client timeout (3s) and a context timeout (1s). Which one fires, and what does the error look like?
4. Test without the network: `httptest.NewServer` with a handler that calls `time.Sleep`. This is the right way to test `checkEndpoint`.

## 7. Notes on the project code

- `checkEndpoint` (`internal/monitor/checker.go`) already follows the context pattern: context passed in, body closed, `LimitReader` used.
- Bug to fix in `CheckAllEndpoints`: `for value := range resultChannel` only ends when the channel is closed, but `close` comes after the loop, so the loop never ends and blocks once all goroutines have sent. Hint: who knows when all goroutines are done? Which `sync` type counts goroutines?
- Minor: the channel is created before checking the `err` from `GetWebsites`. Check `err` right after the call.

## Next topics

- `net/http/httptest` to test `checkEndpoint`.
- `http.Transport` tuning: connection pooling and `MaxIdleConnsPerHost`, relevant when checking many endpoints on the same host.
