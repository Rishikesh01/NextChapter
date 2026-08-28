# 0009 - Site-rule management in the extension, and token hygiene

## Context

ADR-0008 shipped the extension with site rules as a read-only cache and deferred rule
management to the future `web/` library. Real usage immediately hit the gap: capturing on a
site without a rule (manhua.example.net) works via the manual form, but there is no way to teach the
extension the site's URL shape without leaving the extension (swagger/curl). The operator asked
for rule management in the extension.

Separately, the first real connect → disconnect cycle exposed two hygiene gaps: Disconnect only
wipes extension storage, stranding the never-expiring API token it minted (one orphan per
cycle), and abandoned sign-in attempts can leave session rows behind until their 30-day expiry.

## Decisions

1. **Rule creation lives in the capture flow; the options page gets view + delete.** This
   supersedes ADR-0008's "site-rule management belongs to `web/`" consequence for creation and
   deletion; full rule *editing* (arbitrary regex authoring) still belongs to `web/`.

2. **The user never writes a regex.** The popup's manual state grows a collapsible
   "Create a rule from this page" builder: the current URL's path is split into segments, the
   extension pre-guesses which segment is the series slug and which carries the chapter number,
   and the user confirms or re-picks. A pure module (`frontend/lib/rule-builder.ts`) generates
   the rule regex in Go syntax — `(?P<name>…)` groups, literal segments regexp-escaped, the
   chapter's numeric run replaced by `[0-9]+(?:\.[0-9]+)?` with surrounding literal text kept
   (except a variable prefix before a chapter keyword, which generalizes to `[^/]+-` so
   "en-chapter-45.5" doesn't pin the rule to one translation — the comics.example.org default's shape),
   and an optional trailing slash — so a rule built here round-trips through
   `POST /sites/rules` validation and matches what `defaults.go`-style rules look like. The
   draft rule is verified locally by running it through the same `detectPosition` pipeline
   against the current URL before it can be saved.

3. **"Save rule & capture" is one action.** Saving the rule immediately refreshes the
   `siteRules:v1` cache (no TTL wait) and completes the pending capture with the detected
   slug/chapter, so the reward for teaching the extension a site is instant.

4. **Options page: a "Site rules" section** (connected state only) listing rules from
   `GET /sites` — host + regex, with per-row delete behind an inline two-step confirm. Deleting
   also refreshes the cache. No edit UI.

5. **Disconnect revokes its own token.** `Settings` gains `apiTokenId`; the sign-in flow stores
   the minted token's id, and Disconnect calls `DELETE /auth/tokens/{id}` (Bearer,
   best-effort — storage is wiped regardless). The paste-token path has no id (mint responses
   are the only source, and there is deliberately no list endpoint), so pasted tokens are not
   revoked on disconnect; that residue is accepted and this ADR documents it. Leaked sessions
   from abandoned sign-ins are left to their 30-day expiry — the backend owns any earlier
   reaping.

## Consequences

- `packages/api-client` grows `createSiteRule`, `deleteSiteRule` (Bearer channel) and
  `revokeToken`; the extension keeps its one-client rule.
- The rule builder's segment heuristics and generated regexes are pinned by unit tests
  (exact-pattern assertions, round-tripped through detectPosition), and e2e covers the full loop: unmatched page → build rule → save & capture →
  revisit → auto-detected. A disconnect e2e asserts the minted token stops authenticating.
- Hosts remain registrable names (the backend's host validation rejects IP literals) — the
  builder surfaces that error field like any other validation failure.
- The `web/` library remains the place for bulk rule management and regex-level editing.
