# 0003 - Migrations tool: goose

## Context

We need a SQL migration tool. The backend-coder spec offered `golang-migrate` or `goose` as the choice. Both are pure-Go, both support SQLite and Postgres, both can be embedded as a library.

## Decision

**`github.com/pressly/goose/v3`**, used as a library (not a separate CLI in the runtime image).

- Migrations live in `backend/migrations/` as numbered `NNNNNN_description.sql` files with `-- +goose Up` / `-- +goose Down` markers.
- Migrations are embedded via `//go:embed` into the binary and applied on startup, before the HTTP server binds.
- `goose` is invoked through its Go API (`goose.SetBaseFS`, `goose.Up`). The CLI is available for local dev (`go run github.com/pressly/goose/v3/cmd/goose@latest`) but is not required to run the server.

Why goose over `golang-migrate`:

- `goose` accepts a `*sql.DB` directly, matching how we already construct connections for both `modernc.org/sqlite` and Postgres. `golang-migrate`'s driver layer is its own abstraction and adds friction for the dual-DB case.
- Mixed Go/SQL migrations are useful for future data-recompute migrations (e.g. when a site-rule version bump invalidates extracted slugs); we don't need it yet but it's there if we do.
- Embedding `.sql` via `embed.FS` is one line with goose.
- One file per migration with directional markers is easier to scan than golang-migrate's split `.up.sql`/`.down.sql` for the small migrations we have.

## Consequences

- Migrations run on every boot. We accept a startup cost of "scan the goose version table" in exchange for ops simplicity (no separate migrate step).
- Forward-only in production. `Down` migrations exist for local dev and CI rollback tests but we do not call them in prod.
- New migrations require a server restart to apply, which is consistent with the deploy model (single binary, replace and restart).
