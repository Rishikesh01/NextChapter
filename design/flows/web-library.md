# Flow: the web library SPA

The web app (`web/`, ADR-0004) is the browse-and-manage surface: the
extension writes reading progress, the library reads and curates it.
Prototypes live in `design/web/`. Auth is the `nc_session` cookie
(set by login/register, 30 days) — the web app never handles API
tokens except to *mint* them for the extension.

Routes referenced below:

```
/login  /register            auth.html
/                            library.html          (default route)
/library/:id                 series-detail.html  (+ reassign-dialog.html)
                             (NOT /series/:id — that path is the API's
                             GET /series/{id}; see ADR-0010 §4 corollary)
/rules                       rules.html
/settings                    settings.html
```

## 1. Auth

```
any route ──(request → 401)──▶ /login?next=<intended path>
                                   │
              ┌────────────────────┤
              ▼                    ▼
      POST /auth/login      POST /auth/register
        200 → cookie set      201 → cookie set
              │                    │
              ▼                    ▼
      redirect to ?next=…   / (fresh-account empty state)
      (default /)
```

- **Guard:** every data fetch that returns 401 redirects to
  `/login?next=<current path+query>`. After success the app returns
  to exactly where the user was — filters included, since filters
  live in the query string (§2).
- **Login failure (401):** red banner *inside* the card ("Wrong
  username or password."), values kept, focus to the password field
  with its value selected. Deliberately doesn't say which was wrong.
- **Register failure (422):** field-level, never a banner — e.g.
  username "already taken" under the username input, focus moved
  there. (`auth.html`, variants 3–4.)
- Login and register are cross-linked pages, not tabs — a URL you
  can bookmark and a browser-native back/forward story.

## 2. Library: browse and filter

- **Load:** `GET /series?limit=50&offset=0` (+ any active filters).
  Rows are `SeriesSummary`: title, status, rating, tags,
  `highest_chapter`, `entry_count`, `last_captured_at`. Sort:
  `last_captured_at` desc — most recently read first.
- **Filters are URL state:** `?status=reading&tag=action&tag=isekai`.
  Reload, back button, and the post-401 return trip all preserve
  them. Changing a filter resets `offset` to 0.
- **Status select** maps 1:1 to the API's five statuses plus "All"
  (no `status` param). **Tag filter** input commits a chip on
  Enter; chips AND together (the API's repeated-`tag` semantics);
  × on a chip removes that tag and refetches.
- **Load more:** appends the next `offset` page under the grid; the
  caption ("Showing 24 of 57") comes from `items.length` / `total`.
  Explicit button, not infinite scroll — honest scrollbar, native
  feel.
- **Rollup display:** "Read till ch `highest_chapter` ·
  `entry_count` sites", tabular numerals. `highest_chapter: null`
  (zero entries) renders "No chapters yet" — never "ch 0".
- **Empty states:** fresh account → explain the capture loop and
  link to Settings for the token; filtered-to-empty → keep the
  filters visible, offer one-click "Clear filters".
  (`library.html`, variants 2–3.)
- Card click → `/library/:id` (the client route; the API's
  `GET /series/{id}` keeps the raw path — ADR-0010 §4 corollary).

## 3. Series detail: edit lifecycle

`GET /series/:id` → `SeriesDetail` (summary + `entries[]`).
Every editor commits itself with `PATCH /series/:id`, sending only
the changed field.

**Commit on success, not optimistically.** Against a self-hosted
(usually LAN-local) server, PATCH latency is tens of milliseconds —
optimistic UI buys nothing and costs a rollback story. So:

1. The control keeps its new value visually (a `<select>` has
   already flipped natively) and the adjacent `.nc-savehint` shows
   "Saving…".
2. On 2xx: hint shows "Saved" for ~2 s, then clears. App state now
   reflects the server response body.
3. On failure: the control reverts to the last server-confirmed
   value, and the standard error treatment applies (§8). Nothing
   the user typed is lost — for text inputs the rejected value
   stays in the field with the field-level message.

Per editor:

- **Status:** select, `{status}` on change.
- **Rating:** select ★1–★10, `{rating}` on change. Unrated series
  show a quiet "Rate" link-button that swaps to the select
  (`series-detail.html`, variant C). There is **no clear option**:
  the v1 API treats `rating: null` as "leave alone" (SeriesPatch,
  by design). If clearing becomes a requirement it needs an API
  change first — route through architect.
- **Tags:** chips + inline add input. Enter/comma commits, × removes;
  every commit sends the FULL resulting list (`{tags: [...]}` —
  the API is replace-not-diff, and `{tags: []}` legitimately clears
  all). Client-validates `^[a-z0-9][a-z0-9-]{0,31}$` and ≤16 before
  sending; invalid input gets the field-level pattern hint with a
  kebab-case suggestion ("try **slice-of-life**"), no request made.
- **Notes:** textarea + explicit "Save notes" button (multi-line
  content deserves a deliberate commit; a blur-save can eat edits).
  Same savehint lifecycle.
- **Title:** not editable in the v1 design, though `PATCH` supports
  it (noted as an easy v1.1: click-to-edit on the heading).

**Delete series:** danger-zone card, two-step inline confirm — the
button swaps for "Delete "«title»" and N entries?" + Confirm /
Cancel, with **Cancel rightmost in the destructive position and
focused** (double-click and Enter–Enter cancel, never destroy). Esc
cancels. Confirm → `DELETE /series/:id` → navigate to `/`.
The count in the question is the honesty mechanism: deleting a
series deletes its entries.

## 4. Entries: per-site rows

Rows sort by `last_captured_at` desc. Per row:

- **Continue reading** — the row's primary action and the page's
  only solid-accent control class: opens `last_url` in a new tab
  (`target="_blank" rel="noopener"`). No request; the extension
  captures the new position when the user reads on.
- **Edit** — inline form under the row (chapter input accepting
  decimals + monospace URL input). Save →
  `PATCH /entries/:id {last_chapter, last_url}` (changed fields
  only). This is the correction path for a mis-parsed chapter.
  Client-validates chapter as a non-negative number with at most
  one decimal point; failure is field-level.
- **Remove** — two-step inline confirm (same rules as everywhere).
  Confirm → `DELETE /entries/:id`; the "read till ch" rollup in
  the section title refreshes from a refetch.
- **Move** — opens the reassign dialog (§5).

Only one row may have an open edit form or confirm at a time;
opening another reverts the first.

## 5. Entry reassignment (the Move dialog)

(`reassign-dialog.html`.) The one modal in the product — a
cross-entity operation that needs the full series list, launched
from a table row.

```
"Move" on an entry row
        │
        ▼
 [dialog opens] ── GET /series (all pages; search filters client-side)
        │
        ├─ pick existing series ──▶ PATCH /entries/:id {series_id}
        │                                │
        │                     200 ─▶ close; refetch detail; both series'
        │                          rollups (highest_chapter, entry_count,
        │                          last_captured_at) change — the note
        │                          under the list says so up front
        │
        ├─ pick "Create new series: «title»"
        │        │
        │        ├─ POST /series {title}          (201 → new id)
        │        └─ PATCH /entries/:id {series_id: new id}
        │
        └─ Cancel / Esc / scrim click ──▶ close, nothing changed
```

- Focus lands in the search input; Tab is trapped; ↑/↓ + Enter work
  (create row included, current-series row skipped).
- The entry's **current series is listed but inert** ("Current
  series" meta, disabled) — users shouldn't hunt for a row that
  silently vanished.
- The create row prefills from the entry's site title and adopts
  the typed filter text (series-picker rule). Its meta line
  ("Starts at this entry's ch 101") previews the new series'
  rollup.
- Picking a row **is** the action — no second confirm. A move is
  non-destructive and reversible by moving back.
- **Create-new failure handling:** the create path is TWO requests.
  If `POST /series` succeeds but the `PATCH` fails, the dialog
  stays open showing the standard error banner with **Retry**,
  which retries only the PATCH (the series now exists; the create
  row is replaced by the real row for it). This is livable but
  inelegant — see the architect note at the end about an atomic
  `new_series_title` on the entry PATCH.

## 6. Site rules lifecycle

(`rules.html`.) Full CRUD, regex included — this page supersedes the
extension's view-and-delete list; the popup keeps its no-regex
builder for creation-in-context.

- **List:** `GET /sites`. `rules` render as host + pattern (mono,
  truncated, full value in `title`) + group-name mapping line.
  `tracked_hosts` **without** a matching rule render as quiet hint
  rows ("No rule yet — captures here are manual. *Add one*") — a
  host you've captured on manually is exactly the host that most
  needs a rule, and this is where the gap gets fixed. "Add one"
  opens the create form with the host prefilled.
- **Create:** "Add rule" → inline form (host, pattern, slug/chapter
  group names defaulting to `slug`/`chapter`) →
  `POST /sites/rules` → 201 closes the form, row appears.
- **Edit:** row's Edit → same form prefilled →
  `PATCH /sites/rules/:id` with changed fields.
- **Delete:** two-step inline confirm → `DELETE /sites/rules/:id`
  (204). If the host still appears in `tracked_hosts`, the row
  degrades to a no-rule hint row rather than vanishing.
- **422 mapping (field-level, values kept, focus to first invalid):**
  - `host` — duplicate: "«host» already has a rule — edit that one
    instead." / invalid shape (IPs rejected).
  - `chapter_url_regex` — "This pattern failed to compile."
  - `slug_capture_group` / `chapter_capture_group` — "«name» isn't
    a named group in the pattern."
- The extension picks up rule changes the next time its popup opens
  (it refetches `GET /sites`); no push channel exists or is needed.
- **Empty state** (all rules deleted, nothing captured): explain
  what rules are, offer both creation paths.

## 7. Settings: token mint-once, sign out

(`settings.html`.)

```
[label input + Create token]
        │  POST /auth/tokens {label}
        ▼  201 {token: "nca_…"}
[minted: read-only mono field + Copy]
   "Save it now — it won't be shown again."
        │  Copy → clipboard; label flips to "Copied" ~2 s
        ▼  Done (or navigate away)
[back to default; plaintext dropped from memory]
```

- The server stores only a hash; **there is no token list endpoint**
  — the card says "shown once, can't be viewed later" *before* the
  user mints, and the warning line repeats it after. Recovery from
  a lost token is mint-new + re-paste, and the copy says exactly
  that.
- The minted token stays on screen until the user dismisses it —
  no auto-hide timer racing the user's paste.
- The warning uses **amber** (act now), not green — minting isn't
  "done", pasting into the extension is.
- **Sign out** (nav and Account card): `POST /auth/logout` → clears
  the cookie → `/login`. Quiet-danger styling, no confirm — signing
  out is cheap.

## 8. Global error treatments

Exactly three, reused verbatim from the extension's vocabulary:

- **Request failure (network / 5xx):** amber banner at the top of
  the content column — transient, not the user's fault — with an
  inline **Retry** that re-issues the failed request. For failed
  *edits*, the control has already reverted (§3); Retry re-sends
  the user's value.
- **422 validation:** never a banner. Red border on the offending
  field, small red message beneath, focus moved there, values kept.
- **401:** redirect to `/login?next=<current path+query>` (§1). No
  banner — the login page is the message. After re-auth the user
  lands back with filters and scroll target intact.

One hue per state, one banner at a time, user input survives every
error.

## Invariants

- Solid accent is reserved for the screen's primary action:
  Capture in the popup; here, **Continue reading** on entry rows
  and the single page-level action (Add rule / Create token /
  auth submit). Never two competing solid-accent controls in the
  same view region.
- Three type sizes on the web (12 / 13 / 17 — the popup's two plus
  the heading); weight does the rest.
- One modal in the product (Move entry). Everything else edits
  inline.
- Destructive actions are always two-step inline confirms with
  Cancel in the destructive position and focused. No destructive
  modals, no browser `confirm()`.
- Chapter numbers: editable pre-commit wherever shown, decimals
  accepted, tabular numerals always.
- All colors from `tokens.css`; both themes via
  `prefers-color-scheme`; native scrollbars and form controls
  (`color-scheme: light dark`).
- Layout: 1040px content cap, responsive to 360px; tables restack
  to blocks under 680px; the card grid reflows via auto-fill (no
  breakpoint list to maintain).
