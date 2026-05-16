# 0002 - Backend project layout

## Context

The backend-coder agent spec still references a `server/` directory. The README and the locked stack put the backend under `backend/`. Layout needs to be decided once so nobody has to guess where new files go.

## Decision

```
backend/
  go.mod
  go.sum
  cmd/
    nextchapter/             # single binary; main.go wires config -> store -> http -> server
  internal/
    config/                  # env + flag parsing, typed Config struct
    auth/                    # password hashing, token mint/verify, middleware
    users/                   # user domain (create, get, list)
    series/                  # series domain
    entries/                 # per-site reading-position domain
    sites/                   # site rule registry (stubbed in this branch)
    httpapi/                 # gin engine + route registration + handlers
      handlers/              # one file per resource (auth.go, series.go, entries.go, ...)
      middleware/            # auth, request ID, structured logging, recovery
      render/                # error envelope, pagination helpers
    store/
      sqlite/                # modernc.org/sqlite driver wiring
      postgres/              # lib/pq or pgx-stdlib driver wiring (stub for v1)
      queries/               # .sql files consumed by sqlc
      generated/             # sqlc output - do not edit
  migrations/                # numbered SQL files (see ADR-0003)
  tests/
    integration/             # spin a real DB, exercise the HTTP API end-to-end
  sqlc.yaml
  Dockerfile
  .goreleaser.yaml
```

Rules:
- `cmd/nextchapter` is the only `package main`. Everything else lives under `internal/` so it cannot be imported by external code.
- Domain packages (`series`, `entries`, `users`, `auth`) own their business logic and depend only on `store/generated` for persistence. Handlers depend on domains, not on `store/generated` directly.
- `httpapi` is the only package that imports `gin`. Domains stay framework-agnostic.
- `sqlc` writes to `internal/store/generated/`. Hand-written queries live as `.sql` files under `internal/store/queries/` and are checked in.

## Consequences

- The agent spec's references to `server/` are stale; treat `backend/` as the truth. The spec will be cleaned up when we do a broader spec pass.
- Switching DBs is a `cmd/nextchapter/main.go` change: pick the driver based on a `DATABASE_URL` scheme.
- Tests under `tests/integration/` are a separate Go package (`package integration_test`) so they only see the public surface.
