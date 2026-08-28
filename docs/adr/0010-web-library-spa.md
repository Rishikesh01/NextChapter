# 0010 - Web library SPA: stack, cookie auth, and single-binary embedding

## Context

ADR-0004 fixed the shape: a JSON-only backend, the extension under `frontend/`, and a separate
web library SPA under `web/` whose build output is embedded into the Go binary via `//go:embed`
and served from `/`. The extension shipped (ADR-0008/0009) and deliberately deferred entry
reassignment UI, bulk rule management, and regex-level rule editing to the web track. This
milestone delivers the full v1 web library and completes the single-binary self-hosted story.

## Decisions

1. **Stack.** React 19 + Vite + `react-router` v7 (library mode, `createBrowserRouter`) +
   `@tanstack/react-query` v5. New workspace package `web/` (`@nextchapter/web`) reusing
   `@nextchapter/api-client` and the shared design tokens (`design/tokens.css`). TypeScript 5.9
   strict via `tsconfig.base.json`; Vitest for prop-driven component tests; Playwright for e2e.
   All versions pinned exact; never `@latest`.

2. **Cookie auth mode.** The SPA authenticates with the `nc_session` cookie, not a Bearer
   token. `ApiConfig` gains `authMode?: 'bearer' | 'cookie'` (default `'bearer'` — the
   extension is unchanged). In cookie mode, data requests send `credentials: "include"` and no
   `Authorization` header, and the missing-token guard is skipped. Any 401 routes the SPA to
   `/login` (preserving the intended destination).

3. **Embed mechanics.** The embedded assets live in a new package `backend/internal/webui`
   (`//go:embed all:dist` — Go embeds cannot escape the package directory, the backend Docker
   build context is `backend/` only, and `all:` is needed for Vite's underscore-prefixed
   files). A one-line placeholder `dist/index.html` is **committed**: an empty-dir embed is a
   compile error, and the placeholder keeps the backend CI job Go-only and the extension's
   `Dockerfile.test` backend build working. `make -C frontend web-embed` builds `web/` and
   copies `web/dist/*` over the placeholder for release builds; the copied real dist is not
   committed (gitignored except the placeholder), and the overwritten tracked `index.html`
   showing as modified is the deliberate "you have a real build embedded" signal —
   `make -C frontend web-unembed` restores the placeholder.

4. **Serving: everything in NoRoute.** gin v1.12's `StaticFS` at `/` panics against registered
   routes, and sub-path `StaticFS` writes its own 404 header before `NoRoute` runs — so the
   `GET /` placeholder handler is removed and ALL static serving happens inside the NoRoute
   fallback (which runs with engine middleware only; no auth — correct for public assets).
   Discrimination rule: respond with the JSON `not_found` envelope when `Deps.WebUI` is nil,
   the method is not GET/HEAD, or the first path segment is an API prefix
   (`auth`, `series`, `entries`, `sites`, `healthz`, `swagger`); otherwise serve the exact
   embedded file if it exists, else `index.html` with 200 (the SPA client-route fallback).
   Registered API routes never reach NoRoute, so their behavior is untouched. Content-hashed
   `assets/` get `Cache-Control: public, max-age=31536000, immutable`; `index.html` gets
   `no-cache`.

   Corollary: SPA client routes must never share a path with a registered API route — a
   browser navigation to `/series/42` hits the API's `GET /series/{id}` directly (the Lax
   cookie rides along) and renders JSON instead of the app. The series detail page therefore
   lives at `/library/:id`, and any future client route must avoid the API-owned first
   segments.

5. **`NEXTCHAPTER_COOKIE_SECURE`.** The session cookie's `Secure` flag was inferred from the
   CORS allow-list (`isHTTPS`), which silently becomes `false` in same-origin production where
   `NEXTCHAPTER_ALLOWED_ORIGINS` is unset — exactly the deployment this ADR enables. A
   tri-state env override is added: unset keeps the inference; `true`/`false` wins outright.
   Operators serving behind TLS should set it to `true` (README ops note).

6. **CSRF acceptance.** The backend has no CSRF token or Origin check; the cookie is
   `SameSite=Lax` and the SPA is same-origin in production (dev uses the Vite proxy, also
   same-origin). This is accepted for v1 and recorded here; an Origin-check middleware (or
   token CSRF) is the named backend-ADR candidate if a cross-origin deployment is ever
   supported.

7. **Dev story.** The Vite dev server proxies `/auth`, `/series`, `/entries`, `/sites`,
   `/healthz`, `/swagger` to `http://127.0.0.1:8080` — same-origin in dev, so cookies work
   with zero env configuration. The `NEXTCHAPTER_ALLOWED_ORIGINS=http://localhost:5173` CORS
   route remains documented as the alternative. The client's base URL is always
   `window.location.origin`; no URLs are hardcoded in components.

8. **Scope (full v1).** Login/register; series library (status/tag filters, cards with
   `highest_chapter` / `entry_count` / `last_captured_at` rollups); series detail (per-site
   entries, continue-reading links to `last_url`, status/rating/tags/notes editing, delete);
   entry reassignment to an existing or NEW series ("new" = `createSeries` then
   `patchEntry {series_id}` — `EntryPatch` has no title field), entry chapter/URL correction
   and delete; site-rule list/create/edit/delete with regex-level editing (the `web/` half
   ADR-0009 deferred — the extension keeps its no-regex builder); a settings page with a
   mint-extension-token card (plaintext shown exactly once; there is deliberately no token
   list endpoint). Rating cannot be cleared in v1 (API limitation, ADR-0008 noted it).

## Consequences

- The `GET /` plaintext placeholder response is gone; the swag spec and the generated frontend
  types regenerate in the same change (both CI freshness gates).
- Backend release artifacts embed whatever `backend/internal/webui/dist/` holds at build time:
  the real UI after `web-embed`, the placeholder otherwise. The Docker release image requires
  running `web-embed` before `docker build backend/` (its context cannot see `web/`).
- The frontend track's Makefile grows `web-build`, `web-embed`, `web-unembed`,
  `web-test-e2e`, `web-test-e2e-docker`; CI gains a `web (e2e)` job running the pinned-image
  gate; workspace-wide lint/typecheck/test/depcheck cover `web/` automatically.
- Web e2e runs in ONE topology: the real backend binary with the real dist embedded
  (production shape), so every spec also exercises the serving path.
- Future candidates surfaced by this ADR: server-side series search (`GET /series?q=`) once
  libraries outgrow client-side filtering; token list/revocation endpoints for a fuller
  settings page; CSRF hardening per §6.
