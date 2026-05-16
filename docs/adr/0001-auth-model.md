# 0001 - Auth model

## Context

NextChapter is self-hosted. The server holds a user's reading history across many sites; "auth" was called out explicitly by the operator, which means a real credential check, not a lock screen on a single-tenant box.

Two callers need to authenticate:

1. The companion web library (browser, cookie-friendly).
2. The browser extension (background service worker, no cookie jar shared with the web app, cross-origin to the server).

The extension talks to the server from `chrome-extension://...` / `moz-extension://...` origins. Cookies set by the server on its own domain are not automatically attached to extension fetches, and asking users to log in inside the extension popup is more friction than the product wants (the whole capture interaction is supposed to be one click).

## Decision

**Multi-user, password-based, with two session shapes off a single source of truth.**

- **Multi-user from day one.** The schema has a `users` table. v1 ships with a single account created at first boot (see ADR-0006), but every protected row is owned by a user. Multi-user is cheap if we build it in now and impossible to retrofit later.
- **Credential**: bcrypt-hashed password. No magic links (need SMTP), no OIDC (need an IdP), no WebAuthn (need a second-device story). Passwords are boring and work everywhere a self-hosted box runs.
- **Web library auth**: `POST /auth/login` with username + password returns an opaque session token. The web app stores it in an `HttpOnly; Secure; SameSite=Lax` cookie named `nc_session`. Logout deletes the session row.
- **Extension auth**: `POST /auth/tokens` (authenticated via session) issues a long-lived **API token** that the user pastes into the extension's settings once. The extension sends it as `Authorization: Bearer <token>` on every request. Tokens are revocable from the web UI.
- **One table, two shapes.** Both session cookies and API tokens are rows in a single `auth_tokens` table with a `kind` column (`session` or `api`). Same lookup path in middleware, same hashing scheme (SHA-256 of the raw token; we store the hash, not the token). Sessions get a short `expires_at` (30 days, sliding); API tokens get a nullable `expires_at` (default: never) and a human-readable `label`.
- **Token format**: 32 bytes from `crypto/rand`, base64url-encoded, prefixed with `ncs_` for session and `nca_` for API tokens so a leaked token is greppable.

## Consequences

- One middleware function handles both auth shapes. Cookie first, then `Authorization: Bearer`. Both resolve to a `*User` on the request context.
- API tokens are bearer credentials with no rotation in v1. That is acceptable for a self-hosted tool where the user owns the server; we revisit if/when we add a hosted mode.
- We carry a password hashing dependency. `golang.org/x/crypto/bcrypt` is pure-Go, so `CGO_ENABLED=0` is preserved.
- No CSRF token needed for the web app because cookie auth is `SameSite=Lax` and all state-changing endpoints require the cookie. Extension calls go through the bearer token path, which is immune to CSRF by construction.
- No refresh-token dance, no JWT, no key rotation surface. If we ever need stateless auth we revisit; for now every request hits the DB on a single indexed lookup, which is fine at self-hosted scale.
