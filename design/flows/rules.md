# Flow: site-rule lifecycle (create, view, delete)

Site rules make chapter pages auto-detect (`detected` state instead
of `manual`). A rule is `{ host, chapter_url_regex, slug_capture_group,
chapter_capture_group }`, stored per-user on the server. The user
never sees or writes a regex in the extension — the popup's rule
builder generates it, and the options page shows it only as truncated
machine detail.

Surfaces: `components/rule-builder.html` (create, popup) and
`components/rules-list.html` (view + delete, options page). The
create path inside the capture state machine is `flows/capture.md`
§5a; this doc covers the lifecycle around it.

## Create (popup only)

From the manual state, "Create a rule from this page" expands the
inline builder. The user marks which URL path segment is the series
name and which contains the chapter number; the extension generates
the pattern:

- Segments before/between/after the chosen ones are escaped and kept
  literal.
- The series segment becomes `(?P<slug>[^/]+)`.
- In the chapter segment, the last numeric run is replaced with
  `(?P<chapter>[0-9]+(?:\.[0-9]+)?)`; any literal text around it is
  escaped and kept (so `chapter-54` → `chapter-(?P<chapter>…)`).
- The whole pattern is anchored `^…/?$` — tolerant of a trailing
  slash, nothing else.

Group names are always `slug` and `chapter` (letters + digits only —
the wire layer validates `alphanum`). This is the same shape as the
compiled-in defaults in `backend/internal/sites/defaults.go`, and it
round-trips through the same Go-regex → JS-regex translation the
runtime matcher uses (`(?P<` → `(?<`).

**Save rule & capture** is one click for two effects: `POST
/sites/rules` (→ 201), then the normal capture pipeline with the
slug + chapter the new rule extracts from the current URL. Failure
handling — duplicate-host 422, network, other — is specced in
`flows/capture.md` §5a; the invariant is that a failed rule save
never loses the user's capture: the manual form remains one click
away with values intact.

## View + delete (options page only)

The "Site rules" section (`rules-list.html`) renders only when
connected, below the connected summary card. `GET /sites` supplies
the rows (`rules`; `tracked_hosts` is unused here). Each row: host
(semibold) + the stored regex (small monospace, truncated, full value
in `title`) + a quiet Delete.

Delete is a two-step inline confirm — no modal: the row's right side
swaps to "Delete rule?" + **Confirm** (red text) / **Cancel**, with
Cancel occupying Delete's former spot so a double-click cancels;
focus moves to Cancel, Esc cancels, one row in confirm mode at a
time. Confirm → `DELETE /sites/rules/{id}`.

There is no edit UI in v1 (the server's `PATCH /sites/rules/{id}`
exists but is unused by the extension): a wrong rule is deleted here
and recreated from the popup in two clicks, which regenerates the
regex from a live example URL — strictly easier than editing a
pattern by hand.

Deleting a seeded default is allowed and permanent for that account —
defaults are copied at registration and never re-applied.

## Cache: rules must be fresh the moment they change

The popup detects against a `storage.local` cache of `GET /sites`
(15-minute stale-while-revalidate, ADR-0008 §8). Waiting out the TTL
after a mutation would make a just-created rule invisibly *not work*,
so every mutation writes through immediately:

- **Create (popup):** on 201, insert the returned rule into the
  cache (keeping its original timestamp — freshness is never
  extended without a fetch; a background refetch reconciles) before
  the capture proceeds — the next popup open on this site is
  `detected`, no refetch needed.
- **Delete (options):** on 204, refetch `GET /sites` and overwrite
  the cache — the next popup open on that site is honestly `manual`
  again.
- **Duplicate-host 422 on create:** treat as "my cache was stale":
  refetch, overwrite, re-run detection.

## Host constraint

Rule hosts are registrable DNS names — the backend rejects IP
literals (and the e2e fake site runs on `localhost`, not `127.0.0.1`,
for exactly this reason). The popup therefore hides the
"Create a rule from this page" entry point on IP-literal hosts
rather than letting the save fail; the options list never needs to
render an IP row.

## Endpoints

| Call | Used by | Purpose |
|------|---------|---------|
| `GET /sites` | popup (cache refresh), options (list) | rules + tracked hosts |
| `POST /sites/rules` | popup rule builder | create (201; 422 on duplicate host / invalid) |
| `DELETE /sites/rules/{id}` | options rules list | remove (204) |
