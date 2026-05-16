# 0006 - First-run user bootstrap

## Context

ADR-0001 commits us to multi-user with passwords. Self-hosted users need a way to get the first account onto a fresh server without a chicken-and-egg `POST /users` that's either public (registration spam) or requires an admin token that doesn't exist yet.

## Decision

**Two paths, the operator picks one at boot:**

1. **Env-var bootstrap.** If `NEXTCHAPTER_BOOTSTRAP_USERNAME` and `NEXTCHAPTER_BOOTSTRAP_PASSWORD` are set *and* the `users` table is empty, the server creates that user on startup and logs that it did. The variables are read once and not retained.
2. **Open-registration window.** If neither env var is set and the `users` table is empty, the server enables `POST /auth/register` until the first user is created, then disables it. This is logged loudly on every request to the open endpoint so accidental exposure is visible.

After the first user exists, `POST /auth/register` returns `404 Not Found` (not 403 — we do not advertise its existence) unless explicitly re-enabled by a future "invites" mechanism (out of scope for v1).

## Consequences

- One-line Docker quickstart works: `docker run -e NEXTCHAPTER_BOOTSTRAP_USERNAME=... -e NEXTCHAPTER_BOOTSTRAP_PASSWORD=... ...`.
- A user who forgets to set the env vars can still get in via the registration window on first boot.
- No public registration in v1. Multi-user is for households / shared homelabs, not the open internet.
- Adding invites later is purely additive: a new endpoint that mints a one-shot registration token.
