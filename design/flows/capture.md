# Flow: capturing a chapter (popup state machine)

The popup is a single small state machine. It is created fresh on every
toolbar click, decides its state synchronously from local data plus the
active tab's URL, and is usually gone again within two seconds. There is
no background work anywhere in this flow — every network call below is
triggered by an explicit user action inside the popup.

Component prototypes referenced below live in `design/components/`.

```
                     toolbar click
                          │
                          ▼
                      [loading]
                          │
        ┌────────────┬────┴───────┬─────────────┐
        ▼            ▼            ▼             ▼
 [not-configured] [uncapturable] [detected]  [manual]
                                     │         │  │
                                     │         │  │ "Create a rule
                                     │         │  │  from this page"
                                     │         │  ▼
                                     │         │ [rule-builder]
                                     │         │  │ Save rule & capture
                                     │         │  │ (POST /sites/rules,
                                     │         │  │  then capture as usual)
                                     └─ Capture ──┘
                                          │
              ┌───────────────┬───────────┼──────────────┬───────────┐
              ▼               ▼           ▼              ▼           ▼
        [success-        [needs-      [error-       [error-      [error-
         advanced] 200    series] 422  auth] 401     network]     validation]
                              │
                              ▼
                       [series-picker]
                              │
              pick existing / create new → POST again
                              │
                              ▼
                     [success-created] 201
```

## 1. `loading`

- Entered the instant the popup opens. The popup reads stored settings
  (server URL, token) and the active tab's URL, and runs the site-rule
  match. All of this is local — no network — so this state should last
  a few milliseconds and never be *visible* in practice.
- **On screen:** popup shell (header with gear) and nothing else. No
  spinner; if we can't resolve in a frame, something is wrong.
- **Transitions:** exactly one of `not-configured`, `uncapturable`,
  `detected`, `manual`.

## 2. `not-configured`

No server URL or no token stored. (`capture-card.html`, variant 3.)

- **On screen:** "Set up NextChapter" title, one line of explanation,
  primary **Open settings** button. This is the only state where a
  button other than Capture gets the primary style — it is the only
  action available.
- **Transitions:** "Open settings" → opens the options page in a new
  tab and closes the popup. See `flows/onboarding.md`.

## 3. `uncapturable`

Configured, but the active tab is not an http(s) page (`chrome://`,
`about:`, new-tab, a local PDF, an extension page…).
(`capture-card.html`, variant 2.)

- **On screen:** quiet centered empty state — "Nothing to capture
  here" plus one small line telling the user what *would* work. No
  button. The gear in the header remains available.
- **Transitions:** none. The user closes the popup.

## 4. `detected`

Configured, http(s) page, and a site rule matched the URL — we have
`site_host`, `series_slug`, and a chapter number.
(`capture-card.html`, variant 1.)

- **On screen, top to bottom:**
  - Header: `site_host` (small, secondary) + settings gear.
  - Series title: the slug prettified for display
    (`solo-leveling` → "Solo Leveling"; hyphens/underscores to spaces,
    title-cased). Display-only sugar — the raw slug is what gets sent.
  - Small note: "Detected from the page URL — edit if wrong."
  - Chapter row: label + compact editable input, prefilled from the
    URL. Accepts decimals (`45.5`); `inputmode="decimal"`, text
    right-aligned, tabular numerals.
  - Primary **Capture chapter** button, full width, 40px tall.
- **Focus:** on the Capture button, so toolbar-click → Enter captures
  without touching the mouse. Enter anywhere in the card submits.
- **Transitions:** Capture → `capturing`.

## 5. `manual`

Configured, http(s) page, no site rule matched. (`manual-form.html`.)

- **On screen:** header with the read-only host, intro line
  "No rule for this site — fill in the details", stacked fields:
  *Series slug* (monospace — it's an identifier, matched verbatim) and
  *Chapter*, then the same Capture button in the same position and
  size as in `detected` — the core action never moves. Below the
  button, one quiet hint ("A rule lets NextChapter detect chapters
  here automatically.") and the link-button **Create a rule from this
  page**, which expands the inline rule builder (`rule-builder.html`).
- **Entry-point visibility:** the hint + link render only when the
  host is a registrable name (the backend rejects IP literals) *and*
  at least one path segment contains a digit — otherwise no valid
  rule can be built from this URL and the manual form stands alone.
- **Empty state is the default state** here: both fields blank,
  focus in the slug field, Capture disabled until both fields are
  non-empty.
- **Client validation before submit:** slug non-empty; chapter parses
  as a positive number with at most one decimal point. Failures render
  the field-level validation treatment (`status-banner.html`, last
  variant) — never a banner.
- **Transitions:** Capture → `capturing`. "Create a rule from this
  page" → `rule-builder` (anything typed into slug/chapter is kept in
  memory for a possible return trip).

## 5a. `rule-builder` (create a site rule inline)

(`rule-builder.html`.) An inline mode of `manual`, not a separate
screen: the slug/chapter fields swap out and the builder swaps in,
same card, same header. The user never sees or writes a regex.

- **On screen, top to bottom:**
  - Intro "Create a rule for this site" + one small caption.
  - The current page's URL path split into segments, rendered as a
    bordered grid of monospace rows (truncated, full value in
    `title`), each row with two native radios under *Series* /
    *Chapter* column headers. Two radio groups over the same rows —
    exactly one series part, exactly one chapter part.
  - Pre-guess: chapter part = the last segment containing a digit,
    preferring segments with a chapter-ish keyword (chapter/ch/ep) so
    "chapter-3" beats a later "page-2"; series part = the segment just
    before it (else the longest remaining segment). Both radios arrive pre-selected; the guess is
    a head start, not a decision.
  - Live preview well (`aria-live="polite"`): "Will detect:
    `<slug>` · ch `<n>`" — the drafted rule applied to the current
    URL. The chapter is the last numeric run in the chosen segment
    (so `en-chapter-45.5` → 45.5), matching the runtime extractor.
  - Primary **Save rule & capture** — the state's capture button:
    same 40px, same position, same solid accent. One click saves the
    rule *and* completes today's capture.
  - Quiet centered link **Back to manual entry**.
- **Invalid selection:** the chosen chapter segment has no digit run,
  or the same segment is picked for both roles. The preview well
  turns into red field-level feedback ("`manga` has no number — pick
  the part that contains the chapter.") and Save disables. No banner
  — banners are for request outcomes.
- **Preview is not editable** — a deliberate exception to "chapter
  editable wherever shown pre-capture". The line previews what the
  *rule* extracts; editing the number would silently diverge this
  capture from every future detection. A wrong number means a wrong
  selection, or "Back to manual entry", where the chapter input is
  editable as always.
- **Transitions:**
  - **Save rule & capture** → `POST /sites/rules` with the generated
    rule (see `flows/rules.md` for pattern generation).
    - `201` → write the new rule into the cached rules immediately,
      then proceed to `capturing` with the slug + chapter the rule
      extracts. From here everything is the normal pipeline — 422
      needs-series still goes to the series picker, and success reads
      exactly like a `detected` capture.
    - `422` duplicate host (stale cache race — a rule for this host
      already exists) → refetch `GET /sites`, re-run detection; if
      the fresh rule matches the page, continue straight to
      `capturing`. If it doesn't, red banner "A rule for this site
      already exists — manage it in settings" over the manual form.
    - Network failure / 5xx → amber banner with Retry (retries the
      rule POST); the builder keeps its selections.
    - Any other rejection → red banner "Couldn't save the rule";
      builder stays populated, and `Back to manual entry` still
      works — a rule failure never blocks a manual capture.
  - **Back to manual entry** → `manual`, previously typed values
    restored.

## 6. `capturing`

`POST /entries/capture` with `{ site_host, series_slug, last_chapter }`
(plus `series_id` or `new_series_title` when resubmitting from the
picker).

- **On screen:** the current card, with the Capture button disabled and
  its label swapped to "Capturing…". Inputs disabled. No spinner
  overlay; against a local server this resolves in tens of
  milliseconds.
- **Transitions on response:**
  - `200` → `success-advanced` (an entry for this (host, slug) already
    existed and was advanced).
  - `201` → `success-created` (first capture for this key).
  - `422` with the needs-series error → `series-picker` (server needs
    to know which series this new entry belongs to).
  - `422` with a field validation error → `error-validation`.
  - `401` → `error-auth`.
  - network failure / non-JSON / 5xx → `error-network`.

## 7. `series-picker` (first capture for a (host, slug) key)

(`series-picker.html`.) The server said "I don't know this entry — 
whose is it?". The picker's job is to finish the capture in one more
click.

- **On entry:** fire `GET /series` (list is user-scoped; rows carry
  `title`, `highest_chapter`, `entry_count`). While in flight, show
  the loading state: disabled search input + three static skeleton
  rows. No shimmer.
- **On screen once loaded:**
  - Title "Which series is this?" + small context line
    (`slug` on `host`) so the user never loses track of what they're
    assigning.
  - Search input ("Filter your series") — filters client-side as the
    user types.
  - The list, vertical, native scroll, max-height ~320px:
    - Row 0, always: **Create new series: "<title>"** — accent-tinted,
      pinned. Title defaults to the prettified page/series title; once
      the user types in the filter, it adopts the typed text instead.
      Meta line: "Starts tracking at ch <n>".
    - Then one row per existing series: title + meta
      "Read till ch <highest_chapter> · <entry_count> site(s)".
- **Empty states, explicitly:**
  - *User has zero series* (their very first capture ever): the list
    is just the create row. That is correct and needs no extra copy —
    the create row *is* the empty state's call to action.
  - *Filter matches nothing:* create row (with the typed title) +
    small centered "No series match "<query>"."
- **Keyboard:** type to filter; ↑/↓ move selection (create row
  included); Enter picks; Esc returns to the capture card with values
  intact.
- **Transitions:**
  - Pick an existing row → re-`POST /entries/capture` with
    `series_id` → `201` → `success-created`.
  - Pick the create row → re-POST with `new_series_title` → `201` →
    `success-created`.
  - `GET /series` fails → `error-network` (Retry re-fetches the list,
    not the capture).

## 8. Success states (terminal)

(`status-banner.html`, green variants.) Banner renders at the top of
the popup body; the card below stays visible with its inputs disabled.

- `success-advanced` (200): "Advanced **<series>** to ch **<n>**".
- `success-created` (201): "Started tracking **<series>** at ch
  **<n>**".
- No auto-close, no confetti, no animation. The user closes the popup
  (or clicks the toolbar again later). Success must be readable for as
  long as the user wants to read it.

## 9. Error states

- `error-auth` (401) — red banner: "Token rejected — open settings".
  The link opens the options page. The card stays beneath so no typed
  data is lost.
- `error-network` — amber banner (transient, not the user's fault):
  "Couldn't reach your server — Retry". Retry re-submits the identical
  payload. Typed values are never cleared.
- `error-validation` (client-side, or 422 field errors from the
  server) — no banner. Red border on the offending field, small red
  message under it, focus moved to the field.

## Invariants

- The Capture button is the only solid-accent control in the popup,
  always full-width, always 40px, always in the same position across
  `detected`, `manual`, and `rule-builder` (where its label is
  "Save rule & capture" — it is still the capture).
- The user never sees a regex anywhere in the popup.
- Every state fits in a ~380px-wide, <500px-tall popup. Only the
  series list ever scrolls.
- User input survives every error. The only things that clear a form
  are success and closing the popup.
- Two font sizes total (13px body / 12px small) in every state above.
