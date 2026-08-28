# 0008 - Extension stack, auth onboarding, and testing strategy

## Context

The frontend track starts here. ADR-0004 already fixed the shape of the frontend: a JSON-only
backend, the browser extension under `frontend/`, a later web library SPA under `web/`, and a
shared types + client library between the two. The canonical API contract is the swag-generated
spec at `backend/internal/swaggerdocs/swagger.yaml` (Swagger 2.0).

The pipeline agent definitions under `.claude/agents/` predate NextChapter (they describe a
"Tab Tracker" project with an `extension/` directory, a `server/` backend, a shared fingerprint
module, and a hand-maintained `docs/api/openapi.yaml`). None of those exist in this repository.

This ADR records the non-obvious choices for the extension bootstrap milestone so the reviewer
gate has something to check drift against.

## Decisions

1. **Root pnpm workspace, now.** Packages: `frontend/` (`@nextchapter/extension`) and
   `packages/api-client/` (`@nextchapter/api-client`); `web/` joins as a third package later.
   Extracting a shared client after the fact means rewriting imports, lockfile, CI, and
   Makefiles; doing it while `frontend/` is empty costs one directory. `frontend/` remains the
   extension package (the README and ADR-0004 define it that way), so the workspace root is the
   repository root.

2. **Stack: WXT + React 19 + TypeScript 5 strict; Vitest for unit tests; Playwright for e2e;
   pnpm.** WXT generates the MV3 manifests for Chrome and Firefox from one config and owns the
   dev/build/zip loop. WXT auto-imports are disabled (`imports: false`) so eslint and depcheck
   see real imports. Browser APIs go through `wxt/browser` (the maintained successor to raw
   `webextension-polyfill` usage). TypeScript 7 exists but the workspace stays on TS 5.9.x for
   now; migration is a future ADR.

3. **API types are generated, never hand-written.** `swagger2openapi` converts the Swagger 2.0
   spec to an OpenAPI 3 intermediate (gitignored under `packages/api-client/.cache/`), then
   `openapi-typescript` emits types-only output committed at
   `packages/api-client/src/generated/api.ts`. Ergonomic aliases live in `src/types.ts`
   (generated schema names carry Go package prefixes like `models.Series`). CI enforces
   freshness with the same generate-then-`git diff --exit-code` pattern the backend uses for
   its swagger docs.

4. **No background script in v1.** The product promise is that the extension does nothing in
   the background; every behavior is initiated from the popup or options page. MV3 does not
   require a service worker and WXT does not force one. The absence is deliberate, not an
   omission.

5. **Permission model.**
   - `activeTab` — read the invoked tab's URL/title at the moment of the click; avoids `tabs`.
   - `storage` — settings and the site-rule cache.
   - `optional_host_permissions: ["http://*/*", "https://*/*"]` with a runtime
     `permissions.request()` for exactly the user's server origin at options-connect time. The
     server URL is user-configured, so no concrete host can be listed at install; the runtime
     grant exempts extension-page fetches to that one origin from CORS and SameSite blocking.
     Firefox supports `optional_host_permissions` from 128, so
     `browser_specific_settings.gecko.strict_min_version` is `128.0`.
   - A manifest `key` pins a deterministic extension ID so Playwright can navigate to
     `chrome-extension://<id>/popup.html`. It ships in every Chromium build on purpose: it is
     only a public key, and a stable ID is also useful for self-hosted unpacked installs.
   - Test-mode builds (`wxt build --mode test`) additionally inject install-time
     `host_permissions` for `http://localhost/*` and `http://127.0.0.1/*` because Playwright
     cannot click native permission prompts. Production builds never contain these.
   - `options_ui.open_in_tab: true` — `permissions.request()` from the embedded options iframe
     is historically unreliable, and the onboarding form needs the room.

6. **Extension onboarding mints its own API token.** The options page: normalize the server
   URL → request the host permission → `GET /healthz` → sign in (or register) over the cookie
   channel (`credentials: "include"`) → `POST /auth/tokens` (cookie-authenticated) → verify the
   minted `nca_` token with `GET /auth/me` (Bearer) → store `{serverUrl, apiToken, username}`
   in `storage.local` → best-effort `POST /auth/logout`. A "Paste token" fallback tab accepts a
   token minted elsewhere (e.g. via the server's `/swagger` UI) and is surfaced automatically
   if minting fails. The token lives in `storage.local`, never `storage.sync` (it must not
   leave the machine); unencrypted at rest is accepted for a self-hosted tool.

   **Channel-binding guard:** the backend accepts session tokens only via cookie and API tokens
   only via `Authorization: Bearer`, and reads the cookie first when both are present. The API
   client therefore sends `credentials: "omit"` on every Bearer call, so a lingering
   `nc_session` cookie can never shadow the token; only the onboarding calls use
   `credentials: "include"`.

7. **Client-side rule matching translates Go regexes.** Site rules are authored in Go/RE2
   syntax with `(?P<name>...)` groups (see `backend/internal/sites/defaults.go`); JS `RegExp`
   requires `(?<name>...)`. The url-detection module translates `(?P<` → `(?<`, matches
   against `new URL(url).pathname`, and extracts the chapter as the last numeric run of the
   captured group (comics.example.org's group captures a whole segment like `en-chapter-45.5`). A rule
   whose translated pattern fails to compile is skipped, never thrown. The `defaults.go` rules
   are pinned verbatim in the frontend unit-test fixture; changing a default means updating
   both sides.

8. **Optimistic capture.** The popup posts `POST /entries/capture` without a series binding and
   lets the server's upsert semantics decide: `200` means an existing entry advanced; `422`
   with `fields.series_id` means first capture for that (host, slug) — the popup then shows the
   series picker and re-captures with `series_id` or `new_series_title` (→ `201`). Rationale:
   `GET /entries` cannot answer "does an entry exist for this key?" without scanning, and the
   common case (advancing) stays one click. Cached site rules use stale-while-revalidate with a
   15-minute TTL.

9. **e2e runs against real everything.** Chromium via `launchPersistentContext` with
   `--load-extension`, driving the real popup/options pages; a real backend binary (SQLite in a
   temp dir) spawned in Playwright global-setup; a local `node:http` fake chapter site whose
   site rule is created through the real API in Go syntax, exercising the full
   translate-and-match pipeline. (The fake site lives on `localhost`, not `127.0.0.1` — the
   backend's site-rule host validation rejects IP literals.) The QA gate runs in the pinned
   `mcr.microsoft.com/playwright:v<version>-jammy` image via `frontend/Dockerfile.test` (stage
   one builds the backend binary); the image tag and the `@playwright/test` version are a
   single Makefile variable, assert-checked.

   **Firefox e2e is deferred.** Playwright cannot load WebExtensions into Firefox, and a
   `web-ext run` smoke has no assertions. v1 gates `wxt build -b firefox` compiling in CI; this
   amends the QA checklist's "both browsers" requirement. Firefox e2e automation is a named
   follow-up.

## Consequences

- `.claude/agents/frontend-coder.md` and `ui-designer.md` are rewritten, and `linter.md`,
  `qa.md`, `reviewer.md` path-corrected, in the same change as this ADR.
- Out of scope for this milestone, deliberately: site-rule management UI in the extension
  (rules are a read-only cache; management belongs to `web/`), series browsing/reassignment UI
  (a `web/` feature), token-revocation UI, i18n.
- Future ADR candidates that would simplify the extension but require backend changes:
  credential-authenticated token minting (removes the cookie dependency from onboarding) and
  server-side series search (`GET /series?q=`).
