# 0005 - Data model v1

## Context

Two product facts drive the schema:

1. A **series** can have multiple **entries**, one per source site. Entries do not collapse: *Solo Leveling* on Site A at ch 100 and *Solo Leveling* on Site B at ch 110 are two rows under one series.
2. **Manual reassignment** is first class. Moving an entry between series, or splitting an entry off into a new series, must be a single, cheap operation. The series cannot own the entry's identity — the entry's identity is its (site, slug) pair.

The capture model is opt-in click-to-capture. Each click on a chapter page either creates a new entry or advances an existing entry's `last_chapter` and `last_url`. The extension sends a `POST` per click; the server is the source of truth for "what's the highest chapter I've read for this series".

## Decision

### Entities

**`users`** — the account table.
- `id` INTEGER PRIMARY KEY
- `username` TEXT UNIQUE NOT NULL
- `password_hash` TEXT NOT NULL  (bcrypt)
- `created_at` TIMESTAMP NOT NULL DEFAULT now
- `updated_at` TIMESTAMP NOT NULL DEFAULT now

**`auth_tokens`** — sessions and API tokens, unified per ADR-0001.
- `id` INTEGER PRIMARY KEY
- `user_id` INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE
- `kind` TEXT NOT NULL CHECK (kind IN ('session','api'))
- `token_hash` TEXT NOT NULL UNIQUE  (sha256 of the raw token, hex)
- `label` TEXT  (NULL for sessions; user-supplied for API tokens)
- `created_at` TIMESTAMP NOT NULL
- `last_used_at` TIMESTAMP
- `expires_at` TIMESTAMP  (NULL = never)
- INDEX (token_hash)
- INDEX (user_id, kind)

**`series`** — the user-facing grouping.
- `id` INTEGER PRIMARY KEY
- `user_id` INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE
- `title` TEXT NOT NULL  (the user's canonical title for the series)
- `status` TEXT NOT NULL DEFAULT 'reading' CHECK (status IN ('reading','completed','on_hold','dropped','plan_to_read'))
- `rating` INTEGER  (NULL or 1..10)
- `notes` TEXT NOT NULL DEFAULT ''
- `created_at` TIMESTAMP NOT NULL
- `updated_at` TIMESTAMP NOT NULL
- INDEX (user_id, status)
- INDEX (user_id, title)

Tags are a many-to-many in a later milestone (out of scope for this branch). Alternate titles are denormalized inside `entries` for now (each entry remembers the title as seen on its site); a separate `series_aliases` table can be lifted out later without breaking the API.

**`entries`** — one row per (series, site) reading thread.
- `id` INTEGER PRIMARY KEY
- `user_id` INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE
- `series_id` INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE
- `site_host` TEXT NOT NULL  (e.g. `reader.example.com`, lowercased, no `www.`)
- `series_slug` TEXT NOT NULL  (the slug as it appears in the URL; opaque to us)
- `site_title` TEXT NOT NULL  (the title as it appears on the site at capture time)
- `last_chapter` REAL NOT NULL  (REAL because half-chapters: 12.5, 27.1 are common)
- `last_url` TEXT NOT NULL
- `last_captured_at` TIMESTAMP NOT NULL
- `created_at` TIMESTAMP NOT NULL
- `updated_at` TIMESTAMP NOT NULL
- UNIQUE (user_id, site_host, series_slug)
- INDEX (series_id)
- INDEX (user_id, last_captured_at DESC)

Notes on the entry shape:
- **Identity is `(user_id, site_host, series_slug)`**. This is the "same thread, advanced" key. A second capture from the same URL pattern updates the existing row's `last_chapter` (only if higher) and `last_url`.
- **Reassignment is a `series_id` update on the existing row.** No data is destroyed; alternate titles survive via `site_title`.
- **`last_chapter` is REAL** to handle `42.5` style chapters. Comparisons are numeric.
- **No `fingerprint_version` column.** The Tab Tracker fingerprint idea does not apply here; the (host, slug) tuple plays the role of stable identity.
- The "read till chapter: XX" on the series card is `MAX(entries.last_chapter) WHERE series_id = ?`. Computed at read time; cheap with the index above.

### Foreign keys and cascades

SQLite requires `PRAGMA foreign_keys = ON` per connection — the store layer sets it. ON DELETE CASCADE is used so deleting a user is a single statement; ON DELETE for series cascades to entries because an orphan entry has no meaning in this product.

### Timestamps

All timestamps are UTC, stored as ISO-8601 text in SQLite (`TEXT` affinity) and as `TIMESTAMPTZ` in Postgres. sqlc maps both to `time.Time`. The application sets timestamps explicitly; we do not rely on database defaults to avoid drift between SQLite and Postgres semantics.

## Consequences

- A capture call is a simple upsert keyed on `(user_id, site_host, series_slug)`. New entry => assigned to a new or existing series; existing entry => `last_chapter` advanced (monotonic, no rewind) and `last_url` updated.
- Reassignment is `UPDATE entries SET series_id = ? WHERE id = ?`. The old series may now have zero entries; we do not auto-delete empty series (the user may want to keep the metadata).
- Adding tags / aliases later is additive — new tables, no rewrites.
- `last_chapter REAL` means clients must send a JSON number, not a string. The API spec enforces this.
- We accept that monotonic `last_chapter` means re-reads don't show up. v1 product is "where was I", not "what have I read". A read-history table can land later if the product wants it.
