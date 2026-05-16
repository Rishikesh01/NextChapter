# 0004 - Web library UI delivery

## Context

The README promises a "companion web library" on the same server. We need to decide whether:

(a) the Go backend renders HTML server-side (templates, HTMX, etc.), or
(b) the Go backend serves a JSON API and a separate frontend track builds a SPA that the backend embeds and serves as static assets.

This affects backend scope significantly: (a) puts templates, CSRF, asset bundling, and an HTML route surface inside `backend/`; (b) keeps `backend/` as a pure JSON server and adds a `web/` track for the frontend-coder.

## Decision

**(b) JSON-only backend; the web library is a separate SPA track.**

- The backend exposes a single REST/JSON API. The same endpoints serve the web library and (where appropriate) the extension.
- The web library lives under `web/` (separate from `frontend/`, which is the extension). It is a TypeScript SPA, built independently, and its build output is embedded into the Go binary via `//go:embed` and served from `/` as static assets in production.
- In dev, the SPA runs on its own dev server and points at the Go API via CORS allow-list (`http://localhost:5173` etc.). In production, same-origin makes CORS a non-issue.
- For the bootstrap milestone (this branch), no SPA exists yet — the backend serves only the API. A `GET /` returns a one-line placeholder until the web track ships.

Why not server-rendered:
- The product's interesting UI is interactive — drag-to-reassign entries, expandable per-site breakdowns, live search across series. That work in templates is more friction than building it once in TypeScript.
- We already have a TypeScript frontend track (the extension). Sharing types and a small client library between the extension and the web library is cheaper than introducing a templating ecosystem.
- Keeps the backend's responsibilities narrow: persist, validate, authorize, return JSON.

## Consequences

- Backend scope stays purely API + embedded static assets. No html/template, no asset pipeline inside Go.
- The web library gets its own ADR and its own coder track later. Out of scope for `feat/backend-bootstrap`.
- The API is the contract for both the extension and the web library. OpenAPI is canonical for both consumers.
- We pay a small "embed a dist directory" cost at release time. Acceptable; `embed.FS` handles it.
