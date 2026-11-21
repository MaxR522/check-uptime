---
name: sqlite-store
description: Conventions for check-uptime's SQLite persistence layer, schema migrations, and store tests. Use when adding or changing tables, queries, migrations, or the monitor.Store implementation.
---

# SQLite store

## Package and driver

- Code in `internal/store/sqlite`. It implements `monitor.Store`, which stays defined in `internal/monitor` (consumer side). Export the domain types `store` needs from `monitor`.
- Driver `modernc.org/sqlite`, imported for side effects, driver name `"sqlite"`.
- `Open(ctx, path string) (*Store, error)` opens the DB, pings it, applies migrations, returns the store. DSN pragmas: `foreign_keys(1)`, `journal_mode(WAL)`, `busy_timeout(5000)`.
- SQLite allows one writer: `db.SetMaxOpenConns(1)` is acceptable for this app's load and removes `SQLITE_BUSY` surprises.

## Migrations

- Files `internal/store/sqlite/migrations/0001_init.sql`, `0002_...sql`, embedded with `//go:embed migrations/*.sql`.
- Read `PRAGMA user_version`, apply every file with a higher number in order, each in its own transaction that ends with `PRAGMA user_version = N`.
- Never edit an applied migration; add a new file. Ask the user before a destructive change (drop or rename column, drop table).
- Keep `docs/database-schema.md` in sync with the migrations.

## Schema rules

- `INTEGER PRIMARY KEY` ids, `REFERENCES ... ON DELETE CASCADE` from endpoints/expectations/incidents/check_results to their parent.
- Times stored as `TEXT` in RFC 3339 UTC (`time.Time.UTC().Format(time.RFC3339Nano)`), parsed back on read. Nullable times and strings use `sql.Null[T]` at the scan boundary, pointers or zero values in the domain.
- Booleans as `INTEGER` 0/1.
- Indexes: `check_results(endpoint_id, checked_at)`, and a partial unique index `incidents(endpoint_id) WHERE resolved_at IS NULL` to enforce one open incident per endpoint at the DB level.

## Queries

- `database/sql` with `?` placeholders, always the `...Context` variants.
- `defer rows.Close()` and check `rows.Err()` after the loop.
- Wrap errors with the operation: `fmt.Errorf("get open incident for endpoint %d: %w", id, err)`. Map `sql.ErrNoRows` to a domain sentinel (`monitor.ErrNotFound`) so callers do not import `database/sql`.
- Multi-statement writes (create endpoint with its expectations) in one transaction with `defer tx.Rollback()`.
- Loading websites with endpoints and expectations: a few queries plus in-Go grouping is fine; avoid N+1 per endpoint.

## Tests

- Each test opens a fresh DB at `filepath.Join(t.TempDir(), "test.db")` through `Open`, so migrations are tested too.
- Table-driven where it fits. Cover: insert and read back, cascade delete, one-open-incident constraint, resolve incident, results ordered by `checked_at` desc with a limit.
