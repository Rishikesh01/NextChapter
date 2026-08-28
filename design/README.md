# NextChapter — design system

Design source of truth for the browser extension (popup + options
page) and the web library SPA. Everything here is static HTML/CSS
meant to be lifted directly into React components — open any file in
`components/` or `web/` in a browser (light and dark both work; flip
your OS theme to check).

## Index

| File | What it is |
|------|------------|
| [`tokens.css`](tokens.css) | All CSS custom properties: color (light + dark), type, spacing, radii, control sizes. The only place colors are defined. |
| [`components/capture-card.html`](components/capture-card.html) | The popup's main surface: detected state (host, prettified title, editable chapter, Capture button), uncapturable empty state, not-configured state. |
| [`components/manual-form.html`](components/manual-form.html) | Unknown-site fallback: read-only host, slug + chapter inputs, same Capture button, "create a rule from this page" entry point. Empty and filled variants. |
| [`components/rule-builder.html`](components/rule-builder.html) | Inline site-rule builder (ADR-0009): URL path as segment rows with Series/Chapter radio columns, live "Will detect" preview, "Save rule & capture". Collapsed / valid / invalid variants. |
| [`components/series-picker.html`](components/series-picker.html) | Post-422 series assignment: filter input, create-new row (pinned, accent-tinted), series rows with "read till ch X · N sites". Results / filtered-empty / loading states. |
| [`components/status-banner.html`](components/status-banner.html) | Success (advanced / created), auth error, network error with retry, and the field-level validation treatment. |
| [`components/options-form.html`](components/options-form.html) | Options page: server URL + connect + 4 status-line variants, auth tabs (sign in / paste token), connected summary card. |
| [`components/rules-list.html`](components/rules-list.html) | Options page "Site rules" section (connected only): host + truncated regex rows, quiet per-row delete with two-step inline confirm. List / empty / confirm variants. |
| [`flows/capture.md`](flows/capture.md) | The popup state machine: loading → (not-configured \| uncapturable \| detected \| manual) → capture → 200/201/422/401/network, including the rule-builder and series-picker branches and every empty state. |
| [`flows/onboarding.md`](flows/onboarding.md) | Options-page flow: URL → host permission → health check → sign-in/create → auto-minted token → verified → connected; paste-token fallback; failure handling. |
| [`flows/rules.md`](flows/rules.md) | Site-rule lifecycle: regex generation from picked segments, create-from-popup, view/delete from options, write-through cache refresh, hosts-not-IPs constraint. |

## Web library (the `web/` SPA)

The companion web app (ADR-0004 track) shares `tokens.css` wholesale —
same indigo, same light/dark palettes, same 4px scale, same two
weights. What changes is scale: a 1040px content cap instead of 380px,
multi-column grids and tables instead of a single stack, and the 17px
heading size allowed everywhere (one per screen).

| File | What it is |
|------|------------|
| [`web/auth.html`](web/auth.html) | Login + register: centered 360px card, cross-linked pages, 401 in-card banner, 422 field-level (username taken). |
| [`web/library.html`](web/library.html) | Main screen: top nav, status + tag-chip filters (URL-synced), responsive series-card grid (auto-fill, 3→1 col), Load more. Populated / fresh-account / filtered-to-empty variants. |
| [`web/series-detail.html`](web/series-detail.html) | Breadcrumb, heading + self-saving inline editors (status, rating, tag chips, notes), entries table with per-row Continue reading / Move / Edit / Remove, danger zone. Five variants incl. inline edit, remove confirm, unrated, entries-empty. |
| [`web/reassign-dialog.html`](web/reassign-dialog.html) | The product's one modal: "Move this entry to…" with search, series-picker rows, pinned create row, inert current-series row, rollup note. Results + filtered-empty variants. |
| [`web/rules.html`](web/rules.html) | Site rules with full CRUD incl. the raw regex: rules table + "no rule yet" hint rows for tracked hosts, create/edit form with Go named-group help, 422 variants (duplicate host, compile failure), empty state. Supersedes the extension's view-only list. |
| [`web/settings.html`](web/settings.html) | Extension-token minting (label → mint → show-once mono token + Copy + amber warning) and the Account card (signed in as / Sign out). |
| [`flows/web-library.md`](flows/web-library.md) | Routing + auth guard (401 → `/login?next=`), filter/pagination state, the PATCH-per-editor save lifecycle (on-success commit + savehint), reassignment incl. the two-call create path, rule CRUD, token mint-once, and the three global error treatments. |

### How the web app scales the system up

- **Nav, not chrome.** One hairline-bordered bar: brand, three links
  (active = semibold + the options page's 2px accent underline — the
  `.nc-tabs` idiom promoted), username + sign out. No hamburger, no
  icons; three links don't need either.
- **Card grid without breakpoints.** `auto-fill, minmax(290px, 1fr)`
  gives 3 columns at the 1040px cap down to 1 at 360px with no media
  query to maintain. Cards are whole-card anchors; footers align via
  fixed-height title/tag zones; chapter numbers stay tabular.
- **Tables for entries and rules, restacking under 680px.** Per-site
  entries and rules are genuinely columnar; a `<table>` keeps 5+ rows
  scannable, and one media query restacks rows into blocks on
  phones — same markup, no parallel card component.
- **Solid accent = the screen's primary action, generalized.** The
  popup's rule ("only Capture is filled") becomes "one filled control
  class per view region": *Continue reading* on entry rows — the web
  counterpart of Capture, it's the product promise — plus the single
  page-level action (Add rule / Create token / auth submit).
- **Status pills are neutral.** All five reading statuses render as
  gray pills: they're library metadata, not success/error feedback,
  and the green/red/amber tokens stay reserved for state.
- **One modal, everything else inline.** Only entry reassignment
  (cross-entity, needs the full series list) gets a dialog — it
  reuses the series-picker rows verbatim, with a token-pure scrim
  (`--nc-bg` at 65% opacity, correct in both themes). Destructive
  actions keep the extension's two-step inline confirm with Cancel
  in the destructive position and focused.
- **Edits save themselves.** Each detail-page editor PATCHes its own
  field and reports through a quiet inline "Saving… / Saved" hint —
  committed on success, not optimistically (LAN-local servers make
  optimism pointless and rollback code expensive). Notes alone gets
  an explicit Save (multi-line edits deserve a deliberate commit).

## Rationale for the major choices

### Accent: one indigo, used sparingly

- Light `#4f46e5`, dark `#5558d0` fills / `#a3a8ff` text — AA contrast
  in both modes for both fill-with-white-text and text-on-background.
- Indigo is deliberately *not* the browser-chrome blue: the popup sits
  inches from Chrome's own blue UI, and a distinct hue keeps our
  controls from being mistaken for browser furniture while still
  reading as calm and utilitarian.
- Exactly one accent. Green/red/amber exist only as *state* colors
  (success / needs-attention / transient-retryable) and never appear
  decoratively. Within any single banner or status there is exactly
  one hue.

### The Capture button

The single most-clicked control in the product, so it gets structural
privileges nothing else gets:

- The **only** solid-accent-filled element in the popup.
- The **only** 40px-tall control (everything else is 32px).
- Always full-width, always in the same position in both `detected`
  and `manual` states — muscle memory works across states.
- Focused on popup open, so toolbar-click → Enter completes a capture
  with zero pointing.
- The one exception: in the *not-configured* state, "Open settings"
  borrows the primary style because it is the only possible action.

### Type scale: two sizes in the popup

- `13px` body (titles, labels, buttons, banners) and `12px` small
  (hints, meta lines, captions). Weight (400/600) does the hierarchy
  work that extra sizes would otherwise do.
- `17px` heading exists for the options page only — it's a full
  browser tab and can afford one headline.
- `system-ui` stack throughout; `ui-monospace` only for technical
  identifiers (series slugs, tokens), signalling "exact string".
- Tabular numerals on every chapter number so `45.5` and `178` align
  and don't wobble while edited.

### Spacing and radii

- 4px base scale: `4 / 8 / 12 / 16 / 24`. Popup padding is 12px —
  dense, like native menus, not like a web page.
- Radii: 6px controls, 8px cards. No pill buttons except status dots.

### Dark mode

- `color-scheme: light dark` on `:root` so form controls and native
  scrollbars re-skin for free — that's most of the "native feel".
- Every color token has a dark counterpart inside one
  `@media (prefers-color-scheme: dark)` block in `tokens.css`.
  Components never branch on theme; they only reference tokens.
- Dark surfaces are neutral near-blacks (`#1f2023`), state tints are
  darkened and desaturated with lifted foregrounds — no neon-on-black.

### Native feel, on purpose

- No shadows, no gradients, no custom scrollbars, no animation beyond
  120ms background/border transitions (skeletons are static — no
  shimmer). Borders and spacing do all the separation work.
- Popup fixed at 380px wide (tokens: `--nc-popup-width`), everything
  in a single vertical column; only the series list scrolls, capped so
  the popup stays under ~500px tall.
- Trust argument: a tool that stores your reading history on your own
  server should look like part of the browser, not like a landing
  page.

### Site rules without regexes

- Rules are regexes on the wire, but the popup never shows one. The
  builder renders the current page's URL path as monospace segment
  rows with two native-radio columns (*Series* / *Chapter*) — picking
  segments **is** the mental model; the extension generates the
  pattern. Radio-per-segment beat two dropdowns at 380px: the user
  sees the whole URL structure at once, and long segments truncate
  with the full value in `title`.
- The builder is an inline mode of the manual state (fields swap
  out, builder swaps in), never appended below it — one card, one
  source of slug/chapter, popup stays under ~500px.
- Its primary button is "Save rule & capture" with the Capture
  button's full privileges (solid accent, 40px, same position),
  because it *is* the capture — creating the rule is a side effect
  the user gets for free.
- The live "Will detect: `slug` · ch `n`" preview is the honesty
  mechanism: the drafted rule applied to the real URL, with a red
  in-well message (no banner) when the selection can't yield a
  chapter number. It is deliberately not editable — it previews what
  the *rule* extracts; the editable path is "Back to manual entry".
- The options page shows rules as host + truncated regex (small
  monospace — transparency, not interaction) with delete as the only
  action. Delete is a two-step inline confirm, no modal: the row's
  right side swaps to "Delete rule?" + Confirm/Cancel, with Cancel
  placed where Delete was so double-clicks and Enter–Enter cancel
  rather than destroy.

### Empty states are designed, not defaulted

- Zero tracks/series: the series picker's pinned "Create new series"
  row *is* the empty state — a user's first-ever capture goes through
  the same one-click path as their hundredth.
- Filter-to-empty keeps the create row and adopts the typed text.
- Uncapturable pages get a quiet explanation, not an error.
- The manual form's default *is* its empty state: focus placed, button
  disabled until valid.

## Implementation notes for the frontend coder

- Each component file has two labelled CSS sections: **demo harness**
  (the framing that makes the file previewable — don't lift) and
  **component styles** (lift verbatim). Shared primitives
  (`.nc-btn-primary`, `.nc-input`, `.nc-header`, …) are duplicated
  across files so each opens standalone; dedupe them into shared
  components/CSS when porting.
- In the real popup, the `.popup` demo frame corresponds to the popup
  document's `<body>` (no border/radius needed — the browser draws
  the popup chrome). Set `body { width: var(--nc-popup-width); }`.
- All copy in the prototypes is final-intent microcopy, not lorem.
- Endpoints referenced in flows match the backend as of this writing:
  `POST /entries/capture` (200 advanced / 201 created / 422
  needs-series), `GET /series`, `GET /healthz`, `POST /auth/login`,
  `POST /auth/register`, `POST /auth/tokens`, `GET /auth/me`,
  `DELETE /auth/tokens/:id`, `GET /sites`, `POST /sites/rules`
  (201 / 422 duplicate-host), `DELETE /sites/rules/:id` (204).
