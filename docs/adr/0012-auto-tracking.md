# 0012 - Auto-tracking: per-host opt-in, background advance-only capture

## Context

Capturing your place currently costs a toolbar click per chapter. That is one click too many
for the product's core loop: a reader moving through twenty chapters in a sitting clicks twenty
times to record something the extension already knows. The ask is that a chapter page on a site
with a rule should record itself.

The blocker is the permission model, not the code. ADR-0008 §5 chose `activeTab` precisely
*because* it is click-gated: the extension can read a tab's URL only at the moment the user
invokes it, which is why the extension ships with no browsing-history access at all. Reacting to
navigation means seeing URLs without a click, and no amount of engineering avoids that — it is
inherently a permission question.

## Decisions

1. **Per-host, opt-in, requested at runtime.** Auto-tracking is enabled one host at a time, from
   the popup while the user is on that host. Enabling calls
   `permissions.request({ origins: ["https://<host>/*"] })`, which
   `optional_host_permissions: ["http://*/*", "https://*/*"]` (already declared for the server
   origin) makes possible with **no new install-time warning**. The extension therefore never
   gains blanket browsing visibility: it sees exactly the sites the user switched on, and
   revoking the host permission turns it off.

   Rejected: install-time `host_permissions` for `<all_urls>` or for the seeded default hosts.
   Both hand the extension standing access to pages the user never asked it to watch, and both
   would reverse ADR-0008 §5 rather than extend it.

2. **`tabs.onUpdated` in a background service worker — no content script.** Host permission for
   a URL is enough to receive that tab's `url` in `changeInfo`; the `tabs` permission is not
   required, and no code is injected into the page at all. A new `background.ts` entrypoint (the
   extension's first) owns the listener. The capture call must happen here rather than in a
   content script: MV3 content-script fetches are subject to the *page's* CORS, while the
   service worker inherits the extension's host permission for the user's server.

3. **Advance-only: auto-capture never creates.** The backend's capture endpoint needs a
   `series_id` or `new_series_title` when no entry exists for a (host, slug) pair, and answering
   that is a judgement call — which series is this? — that must not be guessed. So auto-capture
   advances entries that already exist and silently declines (422 `needs-series`) otherwise. A
   new series is still adopted through the popup's picker, once; every chapter after that is
   automatic. This is also what makes the feature safe by construction: the worst case is
   nothing happening.

4. **A dwell before recording.** Capture fires only once a tab has held the same matching URL,
   active, for `AUTO_TRACK_DWELL_MS` (5s). Clicking through a table of contents, or opening a
   chapter to check one panel, is not reading, and recording it would move the user's place to
   somewhere they have not been. The dwell is what separates "I am reading this" from "I passed
   through". Navigating away before it elapses cancels the pending capture.

   The backend's monotonic advance (`chapter >= existing.last_chapter`) already makes *backward*
   navigation harmless — re-reading chapter 50 at position 300 records nothing — so the dwell
   exists to guard the *forward* peek, which is the only direction that can mislead.

5. **Silent by default.** Auto-capture reports success by updating the toolbar badge briefly,
   not by a notification: the whole point is that the user stops thinking about capturing. Auth
   failures are the exception — a rejected token disables nothing but surfaces on the badge, so
   a silently broken tracker cannot masquerade as a working one.

## Consequences

- The extension gains its first background entrypoint and its first behavior that runs without a
  user gesture. Anything added there in future inherits that scrutiny.
- Enabled hosts live in `storage.local` alongside the granted permission. The two can drift —
  a user can revoke the permission in browser settings — so the worker treats the *permission*
  as authoritative and the stored list as intent, re-checking with
  `permissions.contains()` before acting.
- The popup's "Capture" button stays exactly as it is. Auto-tracking is additive: it never
  becomes the only way to record a position, and a host with it switched off behaves as before.
- Deferred: auto-tracking a *new* series (needs an answer to "which series?" that only the user
  has); a per-host dwell override; and reacting to in-page navigation on SPA readers that swap
  chapters without a document load — `tabs.onUpdated` fires on history changes, which covers
  most, but a reader that mutates the DOM alone will not auto-track.
