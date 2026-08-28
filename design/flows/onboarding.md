# Flow: onboarding (options page)

Goal: from a fresh install to "Connected to <server> as <username>"
with a working API token stored — in one screen, two steps, and as few
concepts as possible. The user should never need to understand what an
API token is on the happy path.

The options page opens in a full browser tab (from the popup's
"Open settings" button, the popup's gear, or the browser's extension
management UI). Prototype: `design/components/options-form.html`.

```
 Step 1: Server                       Step 2: Account
 ┌──────────────────────────┐         ┌────────────────────────────┐
 │ URL → [Connect]          │  pass   │ Sign in  |  Paste token    │
 │  └ permission prompt     │ ──────▶ │  user/pass → login →       │
 │  └ GET /healthz          │         │  mint token → verify → ✓   │
 └──────────────────────────┘         └────────────────────────────┘
                                                   │ mint fails
                                                   ▼
                                        auto-switch to Paste token
```

## Step 1 — Server

**State:** one URL input + a **Connect** button + a status line.
Step 2 is rendered but visibly disabled ("Connect to your server
first") until Step 1 passes — the ordering is on the page, not in the
user's head.

1. **User enters the server URL** and clicks Connect (or presses
   Enter).
   - Normalize before doing anything: trim, prepend `https://` if no
     scheme given, strip trailing slash and any path.
2. **Host permission grant.** The extension requests optional host
   permission for that origin (`browser.permissions.request`). This
   MUST run in the Connect click handler — browsers only honor
   permission requests inside a user gesture, which is why Connect is
   a button and not an on-blur check.
   - Browser shows its native permission prompt.
   - If the user declines: status line goes to the error variant with
     "Permission declined — NextChapter can't reach this server
     without it". Connect stays enabled; clicking it re-prompts.
3. **Health check.** `GET <server>/healthz` (unauthenticated).
   - Status line while in flight: neutral dot, "Checking…". Connect
     disabled during the check.
   - `200` → green dot, "Server reachable". Step 2 unlocks; focus
     moves to the username field.
   - Anything else (timeout, refused, non-2xx, TLS error) → red dot,
     "Could not reach server — check the URL and that the server is
     running". Step 2 stays locked.

**Status line variants (exactly one visible):**

| variant     | dot     | text                             |
|-------------|---------|----------------------------------|
| unchecked   | neutral | "Not checked yet"                |
| checking    | neutral | "Checking…"                      |
| reachable   | green   | "Server reachable"               |
| unreachable | red     | "Could not reach server — …"     |

Changing the URL after a successful check resets the status to
*unchecked* and re-locks Step 2.

## Step 2 — Account

Two tabs. **Sign in** is the default and the happy path; **Paste
token** is the escape hatch. Tabs, not a wizard: users who already
have a token shouldn't wade through a sign-in form.

### Tab: Sign in (default)

Fields: username, password. Primary button **Sign in**. Below it, a
text link: "New server? Create an account instead" — clicking it flips
the same form into registration mode (button label becomes **Create
account**, the link becomes "Have an account? Sign in instead";
nothing else changes — same two fields).

On submit, the extension runs the whole chain itself; the user watches
one button:

1. `POST /auth/login` (or `POST /auth/register` in create mode) with
   username + password. This yields a session cookie.
   - `401`/`403` on login → field-level error under the password
     field: "Wrong username or password." No banner.
   - Register conflict (username taken) → field-level error under
     username.
2. **Token minted automatically:** `POST /auth/tokens` using the
   session, with a recognizable name (e.g. "browser-extension") so the
   user can identify and revoke it later from the web library.
3. **Verify:** `GET /auth/me` with `Authorization: Bearer <token>` —
   proves the stored credential actually works before we claim
   success, and gives us the username for the connected card.
4. Store `{server URL, token}` in extension storage. Password is never
   stored; the session cookie is not reused after this flow.
5. → **Connected** state.

While the chain runs, the button reads "Signing in…" and is disabled.
The three requests are one gesture from the user's point of view; do
not narrate the intermediate steps unless one fails.

**If minting or verification fails** (login worked but `POST
/auth/tokens` or `GET /auth/me` errored — e.g. an older/modified
server): auto-switch to the **Paste token** tab and show a small
explanatory line above the token field: "Signed in, but couldn't
create a token automatically. Create one from your server's API docs
and paste it here." The user's sign-in wasn't wasted — they now have a
session for the swagger UI linked below.

### Tab: Paste token

For users who minted a token themselves, or landing spot after a
failed auto-mint.

- One field: **API token** (`type="password"`, monospace) + help text:
  "Create one from your server's API docs at `<server>/swagger`
  (`POST /auth/tokens`), then paste it here." The `<server>` link uses
  the URL from Step 1.
- Primary button **Save token**. On click:
  1. `GET /auth/me` with the pasted token.
  2. `200` → store, → **Connected**.
  3. `401` → field-level error: "Token rejected by the server." The
     pasted value stays in the field.
  4. Network failure → status treatment on the Step 1 line (the
     server presumably went away); token field untouched.

## Connected (terminal state)

Both setup sections collapse into one summary card:

- Green dot + "Connected as **<username>**", with the server URL on a
  small second line.
- One **Disconnect** button (outline, red text — destructive but
  quiet). Clicking it deletes the stored token from extension storage
  and returns to Step 1 with the server URL still prefilled.
  - Best-effort: also `DELETE /auth/tokens/:id` if we kept the token
    id from minting; if that call fails, disconnect locally anyway.
- No other settings live here yet. When settings grow, they appear
  below this card; the connection summary always stays on top.

## Re-entry cases

- **Opened while already connected:** show the Connected state
  immediately, then silently re-verify with `GET /auth/me`; on 401,
  drop to Step 2 with a field-level note "Your token was rejected —
  sign in again." Server URL stays prefilled.
- **Arriving from the popup's 401 banner:** same as above — Step 1
  prefilled and already validated, Step 2 active with the note.
- **User changes server URL while connected:** treat as Disconnect +
  fresh Step 1.

## Notes for the frontend coder

- The `/swagger` help link target is `<server>/swagger/index.html`
  (gin-swagger's UI route).
- Token name used for auto-minting ("browser-extension") is a
  UX-visible string; keep it stable so revocation lists read well.
- All copy above is final unless the architect changes product
  behavior; wording lives in the prototypes.
