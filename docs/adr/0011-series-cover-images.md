# 0011 - Series cover images: extension-side acquisition, blob storage, no outbound backend

## Context

The library grid (ADR-0010) identifies a series by title text alone. Readers recognise
manga/manhwa by cover art, so a wall of text cards is the wrong affordance for the one screen
users open most. Every tracked site already displays the cover; the question is how it reaches
the library without compromising the backend's security posture or the single-binary story.

Two facts, established by probing real sites before choosing:

1. A chapter reader page's `og:image` is often a **social card**, not the cover. One large
   catalogue site serves a generated og-image at 1200x630 — a landscape composite, not the
   2:3 portrait art. Auto-detection alone therefore produces wrong-looking covers on real sites.
2. Some sites return **nothing usable to a server-side fetch**. `tapas.io` served no `og:image`
   in its server HTML (client-rendered or bot-gated). A backend scraper gets nothing there; an
   extension reading the live post-JS DOM gets the real image.

Fact 2 rules out server-side scraping on capability grounds before security even enters. But
security is decisive on its own: the backend today makes **zero** outbound HTTP requests (every
`net/http` import is inbound — handlers, middleware, server). Teaching a self-hosted binary
sitting on a home LAN to fetch arbitrary user-supplied URLs turns it into an SSRF pivot toward
router admin pages and cloud metadata endpoints. That surface is currently empty and this ADR
keeps it empty.

## Decisions

1. **The extension acquires the bytes; the backend never fetches.** The extension is already on
   the page with the user's session, so its `fetch` carries the correct `Referer` and cookies —
   which also defeats the hotlink protection common on manga hosts, something a hotlinked
   `<img src>` in the web UI cannot do. It uploads the bytes to the API. The backend's outbound
   HTTP surface stays exactly zero; there is no URL-fetching endpoint, now or as a fallback.

2. **One acquisition path: the user picks.** On any page — and the one that matters is a
   series' chapter-list / home page, where the real portrait art lives — the user opens the
   popup, chooses the series, and picks the cover from a thumbnail grid of the images actually
   present on that page. Ranking floats the likely cover to the front (declared artwork, then
   portrait, then largest); the eye does the rest.

   An *automatic* variant was specified and then dropped: on capture, read `og:image` and
   upload it if the series has no cover yet. It was cut because the two facts above make it
   actively bad rather than merely imperfect — on a chapter page `og:image` is usually the
   1200x630 social card, so the automatic path's most common outcome is silently installing a
   wrong-shaped cover that the user then has to notice and undo. It also costs a round trip on
   every capture to discover whether a cover already exists. Picking is one extra click, once
   per series, and it is always right. Revisit only with evidence that `og:image` is the real
   portrait art on the sites people actually use.

   The picker is deliberately reachable on pages that match no chapter rule, so a series home
   page — previously a dead end in the popup — becomes useful.

3. **Storage: a separate `series_cover` table holding a BLOB**, one row per series, not a column
   on `series`. Keeping the bytes out of `series` means the list query — the hottest path in the
   product — never drags image data through a `SELECT *`. Covers run ~50-200KB; a few hundred
   series stays well under 50MB, which SQLite handles comfortably and which preserves the
   "one binary, one file to back up" property. Filesystem storage would be faster but adds a
   volume path the Docker story and every backup instruction would have to carry.

4. **Trust the bytes, not the client.** The upload endpoint sniffs the content type from the
   bytes themselves (`http.DetectContentType`) and rejects anything outside JPEG / PNG / WebP
   regardless of the declared `Content-Type`. Size is capped at 5MB via `http.MaxBytesReader` so
   a hostile client cannot exhaust memory, and the decoded pixel dimensions are stored for the
   UI to reason about aspect ratio.

5. **Serving: `GET /series/{id}/cover`, cookie- or Bearer-authenticated like every other route.**
   The SPA renders `<img src="/series/{id}/cover?v=...">`; same-origin means the `nc_session`
   cookie rides along, so no separate signed-URL scheme is needed. Responses carry a strong
   `ETag` (the stored bytes' hash) and `Cache-Control: private, max-age=0, must-revalidate`, so
   a repeat library view costs a 304 rather than a re-download. `private` matters: covers are
   per-user data and must never land in a shared proxy cache.

6. **`cover_updated_at` on the series summary is the existence flag and the cache-buster.** Null
   means no cover, so the grid renders a placeholder without issuing a doomed request per
   coverless series; non-null feeds the `?v=` query param so a replaced cover busts the cached
   image immediately.

7. **Reading the bytes: page context first, host permission as fallback.** The chosen image
   is fetched by injected code running *in the page*, so the request carries that page's
   cookies and looks like the site loading its own artwork. That fails when the image sits on
   a CDN that sends no CORS headers — common — so the fallback asks for the optional host
   permission on the image's own origin and fetches from the extension, where host permissions
   exempt the request from CORS entirely. Only the image the user actually picked is ever
   fetched; the thumbnail grid renders straight from `<img src>`, which needs neither CORS nor
   a permission. Residual gap: the fallback path sends no `Referer`, so a host that hotlink-
   checks *and* blocks CORS can still refuse — the picker reports that and the user picks
   another image.

8. **The extension gains the `scripting` permission.** It currently holds only `activeTab` +
   `storage` and reads just `tab.url` / `tab.title` — it has no DOM access at all. Reading
   `og:image` or enumerating candidate images requires `chrome.scripting.executeScript`. This
   adds no new install-time host warning beyond what `activeTab` already implies, and injection
   still happens only on an explicit user gesture, preserving ADR-0008 §5's posture: no broad
   host permissions, nothing runs until the user clicks.

## Consequences

- Migration `000007_series_covers` lands in both engines; the series list/summary queries grow a
  `cover_updated_at` correlated subquery, and sqlc regenerates for both `generated/` and
  `generated/pg/`.
- The API grows three routes under the already-API-owned `/series` prefix (ADR-0010 §4 keeps SPA
  client routes clear of it): `PUT`, `GET` and `DELETE /series/{id}/cover`.
- `DELETE /series/{id}` must cascade to `series_cover`; the FK carries `ON DELETE CASCADE`.
- The backend's zero-outbound-HTTP property is now a **documented invariant**, not an accident.
  Any future "fetch this URL for me" feature reopens the SSRF question and needs its own ADR.
- Covers are per-user rows: two users tracking the same title each store their own bytes. No
  dedup by content hash in v1 — it would couple users' storage together for a saving that does
  not matter at self-hosted scale.
- The web UI can *show* and *remove* a cover but not set one: acquisition needs a page to read
  it from, which only the extension has. A local-file upload from the browser would reuse the
  same endpoint unchanged and is the obvious next step if that dead end bites.
- Deferred: cover art for series created from the web UI with no extension installed (no
  acquisition path exists for them — they show the placeholder); server-side thumbnailing or
  re-encoding (bytes are stored as uploaded); and content-hash dedup.
- Surfaced while building this: the SQLite driver returns `MAX()` over a TIMESTAMP column as
  TEXT in Go's `time.Time.String()` layout, which the repository's conversion shim could not
  parse — so `last_captured_at` had silently been null on every series summary since the
  rollup shipped, and two integration tests had pinned that null as expected. Fixed with the
  cover rollup, since both ride the same code path.
