# Streams in Go (`io.Reader`, `io.Writer`)

Lesson from 2026-10-29. Explains what "`resp.Body` is a stream" means, and how streams work in Go.

## 1. What a stream is

A stream is data you read piece by piece, in order, as it becomes available, instead of having all of it in memory at once.

Analogy: a `[]byte` is a bucket (everything is there, you can look at any part as often as you want). A stream is a pipe (data comes out as it flows; once it has passed, it is gone).

### Analogy with channels

If you know channels, you already know the idea of a stream. Picture the check-uptime app: each endpoint check runs in a goroutine and sends its result to a channel, and one function receives the results as they arrive (fan-in). That channel behaves like a stream, even though it does not implement `io.Reader`:

- Something produces data over time.
- The consumer takes it in order, as it arrives, and waits when nothing is there yet.
- Once taken, it is gone.
- There is an "it's over" signal.

So "`resp.Body` is a stream" means: think of the body as a channel of bytes fed by the network. `io.ReadAll` is like a `for v := range ch` loop that appends everything to a slice. Reading a second time gets nothing, just like ranging again over a drained, closed channel.

| | `io.Reader` | channel |
|---|---|---|
| Order | In order | In order (FIFO) |
| Consumed once | Read bytes are gone | A received value is gone |
| Waits for data | `Read` blocks until bytes arrive | `<-ch` blocks until a value is sent |
| End signal | `io.EOF` | `close(ch)`; `range` ends, or `v, ok := <-ch` gives `ok == false` |
| Backpressure | A slow reader slows the TCP sender | A slow receiver blocks senders (unbuffered or full channel) |

Where the analogy stops:

1. **Unit of data**: a Reader gives raw bytes in chunks whose boundaries mean nothing (one JSON object can arrive across 3 `Read` calls). A channel gives whole typed values: one value is one receive, never half of one. That is why `bufio.Scanner` and `json.NewDecoder` exist: they rebuild lines or objects from byte chunks.
2. **Concurrency**: channels are built for communication between goroutines, with many senders and receivers and synchronization built in. A Reader is usually **not** safe for concurrent use: one goroutine reads it.
3. **Errors**: `Read` returns an error. A channel has no error path, so you put the error inside the value (a result struct with an error field).
4. **Who closes, and what it means**:
   - Reader: the **consumer** calls `Close()` ("I'm done, free the resource"), as with `resp.Body`.
   - Channel: the **sender** calls `close(ch)` ("no more values will come"). A receiver never closes a channel, and sending on a closed channel panics.

`io.Pipe()` bridges both worlds: it returns a connected `*io.PipeReader` and `*io.PipeWriter`, with the same blocking hand-off between goroutines as an unbuffered channel.

## 2. The core: `io.Reader`

In Go, a stream is anything that implements one method:

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}
```

The contract:

- You pass a buffer `p`. `Read` fills **up to** `len(p)` bytes and returns `n`, the number of bytes actually written.
- `n` can be smaller than `len(p)` even when more data will come later. This is normal, not the end of the data.
- When there is no more data, `Read` returns `io.EOF`. It is a signal, not a failure.
- `Read` can return `n > 0` **and** an error in the same call. Process the `n` bytes first, then look at the error.

The manual read loop:

```go
buf := make([]byte, 4096)
for {
    n, err := r.Read(buf)
    if n > 0 {
        process(buf[:n]) // only the first n bytes are valid
    }
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
}
```

You rarely write this loop yourself: `io.ReadAll`, `io.Copy`, `bufio.Scanner` and `json.NewDecoder` do it for you. Knowing that this is what they do inside explains everything else in this lesson.

## 3. The other direction: `io.Writer`

```go
type Writer interface {
    Write(p []byte) (n int, err error)
}
```

`io.Copy(dst io.Writer, src io.Reader)` connects the two: it reads from `src` and writes to `dst` until EOF, using a small buffer. Writers include `os.Stdout`, files, `bytes.Buffer`, `http.ResponseWriter`, and `io.Discard` (throws everything away).

## 4. `resp.Body` specifically

`resp.Body` is an `io.ReadCloser`: `Reader` plus `Close() error`.

When `client.Do` returns, Go has received **only the status line and the headers**. The body is still on its way over the TCP connection, or the server has not even sent it yet. Each `Read` pulls the next bytes from the network.

This explains the HTTP rules:

| Rule | Why, in stream terms |
|---|---|
| Read it only once | No rewind. A second `io.ReadAll` returns empty, the bytes were already consumed. |
| Close it | `Close` gives the connection back to the pool. While the body is open, the connection is busy. |
| Drain it (`io.Copy(io.Discard, resp.Body)`) | Unread bytes are still on the connection. It can only be reused once the stream is at its end. |
| Use `io.LimitReader` | The stream length is unknown: 5 GB, or never-ending. `io.ReadAll` would grow memory without bound. |
| A read error is a real failure | The connection can drop in the middle of the body. `client.Do` succeeded, but `Read` fails later. |
| `Client.Timeout` covers the body | The timer keeps running after `Do` returns, because reading the body is still network work. |

The read-twice pitfall:

```go
body1, _ := io.ReadAll(resp.Body) // all the bytes
body2, _ := io.ReadAll(resp.Body) // empty: the stream is consumed
```

If you need the data twice, read it once into a `[]byte` and reuse that slice.

## 5. Composition: readers wrapping readers

Many functions take an `io.Reader` and return another `io.Reader`, so you can chain them like pipes:

```go
limited := io.LimitReader(resp.Body, 1<<20) // stops after 1 MiB
data, err := io.ReadAll(limited)
```

Useful ones:

| Function | What it does |
|---|---|
| `io.LimitReader(r, n)` | Returns EOF after `n` bytes. |
| `bufio.NewScanner(r)` | Reads line by line (or word by word). |
| `json.NewDecoder(r).Decode(&v)` | Decodes JSON while reading. |
| `io.TeeReader(r, w)` | Everything read from `r` is also written to `w` (logging, hashing). |
| `gzip.NewReader(r)` | Decompresses on the fly. |
| `io.MultiReader(r1, r2)` | Reads `r1` then `r2` as one stream. |

Because `resp.Body`, `*os.File`, `os.Stdin`, `bytes.Buffer` and `strings.NewReader("...")` are all Readers, the same code works on all of them. That is also why tests can use `strings.NewReader` in place of a real network body.

## 6. Why streams exist

1. **Memory**: process a 10 GB file with a 4 KB buffer.
2. **Latency**: start working before all the data has arrived.
3. **Unknown or unlimited size**: network sockets, logs, stdin.
4. **One interface for everything**: files, network, memory, compression all use the same `Read`.

The cost: read once, forward only, and you must handle EOF, errors and closing.

## 7. Stream or `[]byte`?

- Convert to `[]byte` (`io.ReadAll`, behind a `LimitReader`) when you need the whole content at once: searching a substring, comparing, reading it twice.
- Stay with the stream when you can process piece by piece: copying to a file, decoding JSON, scanning lines, hashing.

In the project, `checkEndpoint` does `io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))`: a stream wrapped in a stream, then turned into a bucket. Right for a body-content expectation, which needs the whole text (up to the limit) to search in.

## 8. Checklist

- [ ] Process `buf[:n]`, never the full `buf`, after a `Read`.
- [ ] Handle `n > 0` before checking `err`.
- [ ] Treat `io.EOF` as the normal end, not as an error to report.
- [ ] Never read a stream twice. Keep a `[]byte` if you need the data again.
- [ ] Put a `LimitReader` in front of `io.ReadAll` for data you do not control.
- [ ] Close every `io.ReadCloser` you open (`defer x.Close()` after the error check).
- [ ] Accept `io.Reader` in your own functions instead of `*os.File` or `*http.Response`: easier to reuse and test.

## 9. Exercises

1. Wrap `strings.NewReader("hello world")` in `io.LimitReader(r, 5)`. Call `io.ReadAll` twice on the result and check what each call returns.
2. Write a custom Reader that returns 1 byte per `Read` call. Check that `io.ReadAll` still gets the full content. This shows why `n < len(p)` is normal.
3. Use `io.TeeReader` to read a string while counting its bytes into a `bytes.Buffer` or a hash (`sha256.New()` is an `io.Writer`).
4. Write a function `containsText(r io.Reader, want string) (bool, error)` and test it with `strings.NewReader`, without any HTTP.
