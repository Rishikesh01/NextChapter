# NextChapter

**Where was I?** A self-hosted progress tracker for manhwa, manhua, and web novels — the things you read across half a dozen scanlation and translation sites and lose your place in.

## The problem

Serialised reading is messy. A single series shows up under different titles, on different sites, in different translations. You start *Solo Leveling* on one aggregator, the chapter you want isn't there next week so you finish it on another, and now your bookmarks are split across three tabs in two browsers on two devices. Existing trackers (AniList, MAL) are catalogues, not progress tools, and they don't know what site you actually read on.

## What NextChapter does

- **Opt-in capture.** The extension does *nothing* in the background. When you're on a chapter page and want to record where you are, you click the extension button. That's the entire capture interaction.
- **Per-site tracking.** Every entry remembers both the *chapter number* and the *site you read it on*. If you read *Solo Leveling* up to ch 100 on Site A, then continue to ch 110 on Site B, NextChapter keeps both threads — it doesn't collapse them.
- **Manual series reassignment.** Same series, different names: *The Beginning After The End* on one site is *TBATE* on another, possibly *오로지 너로 시작되는* on a third. You can grab any entry and reassign it to a different (or new) series. Manual override is a first-class feature, not a debugging tool.
- **URL-pattern heuristics, with fallback.** For known sites we extract series-slug and chapter number from the URL automatically (`/series/<slug>/chapter-<n>` and similar). For unknown sites, you fill in the details yourself.
- **Companion web library.** Beyond the extension there's a web app on the same server. List your series, mark them as *reading / completed / on-hold / dropped / plan-to-read*, tag them, rate them, and click straight back into the last chapter you read. Each series card shows `read till chapter: XX` (the highest chapter across all sites you've used for it) and expands into the per-site breakdown.
- **Self-hosted.** Your reading list is yours. The server is a single Go binary or container — run it on a laptop, a Pi, a VPS. Your data never leaves machines you own.

## Stack

### Backend (`backend/`)

- Go 1.26.
- `github.com/gin-gonic/gin` for HTTP routing.
- `sqlc` for type-safe SQL.
- `modernc.org/sqlite` (pure-Go) by default; Postgres as a production option.
- `CGO_ENABLED=0` everywhere — pure-Go dependencies only.

### Frontend (`frontend/` + `packages/`)

- TypeScript 5.x, strict mode; React 19 on WXT.
- Manifest V3 for Chrome and Firefox (v128+, for optional host permissions).
- Browser APIs via `wxt/browser` (cross-browser).
- pnpm workspace rooted at the repo root; `packages/api-client/` is the shared, generated-types API client (shared by the extension and the `web/` SPA).
- Playwright for e2e tests (real browser, real backend binary — see ADR-0008), Vitest for unit tests.

## Status

**Backend — implemented.** `backend/` is a complete Go JSON API: open registration, session-cookie and API-token auth, and CRUD for series (with tags and ratings), per-site reading entries, and site URL-rules. It runs on SQLite (pure-Go, the default) or Postgres, with goose migrations and sqlc-generated queries maintained for both engines. Tests run against real databases (testcontainers for Postgres) across a dual-engine CI matrix.

**Extension — implemented.** `frontend/` is the MV3 extension: options-page onboarding (connect to your server, sign in, a token is minted automatically), one-click capture from the popup — site rules parse the chapter from the URL, unknown sites get a manual form with a no-regex rule builder, and first-time captures go through a series picker. Design tokens and component specs live in `design/`.

**Web library — implemented.** `web/` is the companion SPA (ADR-0010): sign in with the same account, browse the library with status/tag filters, open a series for its per-site breakdown and click straight back into the last chapter, edit statuses/ratings/tags/notes, move entries between series (or onto a new one), manage site rules down to the regex, and mint extension tokens. In production it is **embedded into the Go binary** and served from `/` — run `make -C frontend web-embed` before `make -C backend build` (or before `docker build backend/`) and the single binary serves both the API and the UI. Dev loop: `make -C backend run` plus `pnpm --filter @nextchapter/web run dev` (the Vite dev server proxies API paths to `:8080`, so cookies just work).

Operator note: the session cookie's `Secure` flag is inferred from `NEXTCHAPTER_ALLOWED_ORIGINS`, which same-origin deployments leave unset — behind TLS, set `NEXTCHAPTER_COOKIE_SECURE=true` explicitly (ADR-0010).

## License

See [LICENSE](LICENSE).
