---
name: verify
description: Run the check-uptime quality gate (gofmt, templ generate, build, vet, race tests) and report failures. Use at the end of every code change, before saying a step is done, and before a commit.
---

# Verify

Run these in order from the repo root. Stop at the first failing stage, fix the cause, then rerun from the start.

1. `gofmt -l .` must print nothing. If it lists files, run `gofmt -w` on them.
2. If any `.templ` file exists: `go tool templ generate`, then confirm `git status` shows no unexpected changes in `*_templ.go` beyond the ones from this change.
3. `go build ./...`
4. `go vet ./...`
5. `go test -race -count=1 ./...`

Report in a few lines: each stage pass/fail, and for a failure the shortest decisive line of output plus the fix applied. Never report "done" with a failing stage; if a failure cannot be fixed within the change's scope, say so plainly.

A test that hits the network (real websites) is a bug in the test: use `httptest.Server` instead.
