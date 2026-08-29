# NextChapter

**Where was I?** A self-hosted progress tracker for web novels, manhwa and manhua — the things you read across half a dozen different sites and lose your place in.

One Go binary serves the API and the web library; a browser extension captures your position as you read. Your data never leaves machines you own.

![The NextChapter library](docs/screenshots/library.jpg)

---

## The problem

Serialised reading is messy. A single series shows up under different titles, on different sites, in different translations. You start something on one platform, pick it up on another where the translation runs further ahead, and now your place is split across three tabs in two browsers on two devices. No single platform tracks across the others — each one only knows its own catalogue — and general trackers like AniList and MAL are catalogues too, not progress tools: they don't know what site you actually read on.

## What NextChapter does

- **Opt-in capture.** Click the extension button on a chapter page to record where you are. Nothing runs in the background unless you turn on auto-tracking for a specific site, one host at a time (ADR-0012).
- **Per-site tracking.** Every entry remembers both the *chapter number* and the *site you read it on*. Read a series to ch 1402 on one site then continue to ch 1450 on another, and NextChapter keeps both threads — it doesn't collapse them.

  ![Per-site breakdown for one series](docs/screenshots/series-detail.jpg)

- **Manual series reassignment.** *The Beginning After The End* on one site is *TBATE* on another, possibly *오로지 너로 시작되는* on a third. Grab any entry and reassign it to a different (or new) series. Manual override is a first-class feature, not a debugging tool.
- **URL-pattern heuristics, with fallback.** For known sites the series slug and chapter number come out of the URL automatically. For unknown sites you fill in the details, or build a rule without writing a regex — and any site you have captured from but have no rule for is surfaced with a one-click prompt to add one.

  ![Site rules, with tracked hosts that have no rule yet](docs/screenshots/site-rules.jpg)

- **Companion web library.** List your series, mark them *reading / completed / on-hold / dropped / plan-to-read*, tag them, rate them, and click straight back into the last chapter you read. Each card shows `read till chapter: XX` — the highest across every site you've used — and expands into the per-site breakdown.
- **Self-hosted.** A laptop, a Pi, a VPS. SQLite by default, Postgres if you'd rather.

---

## Install

### Docker (recommended)

The image bundles the web UI — one container serves the API and the library.

```sh
docker run -d --name nextchapter \
  -p 8080:8080 \
  -v nextchapter-data:/data \
  -e NEXTCHAPTER_DATABASE_URL=sqlite:///data/nextchapter.db \
  ghcr.io/rishikesh01/nextchapter:latest
```

Open <http://localhost:8080>, register an account, and you're running. Images are multi-arch (`linux/amd64`, `linux/arm64`, `linux/arm/v7`) and run as a non-root user on a distroless base. Every tag is also published as `:latest` for stable releases, and carries OCI metadata — `docker inspect` reports the exact version and commit it was built from.

### Binary

Grab the archive for your platform from the [latest release](https://github.com/Rishikesh01/NextChapter/releases/latest), check it against `checksums.txt`, extract, and run:

```sh
tar -xzf nextchapter_v1.2.3_linux_amd64.tar.gz
cd nextchapter_v1.2.3_linux_amd64
./nextchapter
```

It listens on `:8080` and creates `./nextchapter.db` next to itself. The web UI is embedded — no separate web server, no static files to copy. Builds ship for Linux (amd64, arm64, armv7), macOS (Intel, Apple silicon) and Windows.

### From source

Needs Go 1.27+, Node 24+ and pnpm 10+.

```sh
git clone https://github.com/Rishikesh01/NextChapter.git
cd NextChapter
make setup                       # Go modules + pnpm workspace
make -C frontend web-embed       # build the SPA into the binary
make -C backend build            # → backend/bin/nextchapter
```

Skip `web-embed` and you get an API-only binary that serves a placeholder at `/` instead of the library. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development setup.

### The extension

Not on the Chrome Web Store or AMO — this is a self-hosted tool, so it installs by hand. Download `nextchapter-extension-<version>-chrome-mv3.zip` or `-firefox-mv3.zip` from the [latest release](https://github.com/Rishikesh01/NextChapter/releases/latest).

- **Chromium** (Chrome, Edge, Brave): unzip it, open `chrome://extensions`, enable *Developer mode*, click *Load unpacked*, pick the folder. The extension ID is pinned, so it survives reloads.
- **Firefox 128+**: open `about:debugging#/runtime/this-firefox` → *Load Temporary Add-on* → pick the zip. (128 is the floor: below it, optional host permissions are silently dropped.)

Then open the extension's options page, enter your server URL, and sign in — it mints its own API token. The extension asks for permission to reach your server's origin at that moment, and asks for nothing at install time.

---

## Configuration

Every setting is an environment variable. No flags, no config file.

| Variable | Default | Meaning |
| --- | --- | --- |
| `NEXTCHAPTER_LISTEN_ADDR` | `:8080` | Address to bind (`host:port`). |
| `NEXTCHAPTER_DATABASE_URL` | `sqlite://./nextchapter.db` | `sqlite://<path>` or `postgres://user:pass@host:5432/db`. |
| `NEXTCHAPTER_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. |
| `NEXTCHAPTER_ALLOWED_ORIGINS` | *(unset)* | Comma-separated CORS allow-list. Unset means same-origin only — correct for the embedded UI. |
| `NEXTCHAPTER_COOKIE_SECURE` | *(inferred)* | Forces the session cookie's `Secure` flag. **Set this to `true` behind TLS** — see below. |
| `NEXTCHAPTER_BOOTSTRAP_USERNAME` | *(unset)* | Creates the first user on an empty database. Must be set together with the password. |
| `NEXTCHAPTER_BOOTSTRAP_PASSWORD` | *(unset)* | At least 8 characters. |
| `NEXTCHAPTER_VERSION` | *(linker-stamped)* | Overrides the version reported by `/healthz`. |

Registration is open regardless of bootstrap — anyone who can reach the server can create an account, so put it behind a network you control or a reverse proxy that authenticates.

> **Behind TLS, set `NEXTCHAPTER_COOKIE_SECURE=true`.** The session cookie's `Secure` flag is otherwise inferred from `NEXTCHAPTER_ALLOWED_ORIGINS`, which a same-origin deployment leaves unset — so a TLS deployment that doesn't set this ships a cookie without `Secure` (ADR-0010 §5).

`GET /healthz` reports `{"status":"ok","version":"v1.2.3"}` — the version is stamped at build time, so it tells you exactly which release is running. Interactive API docs are at `/swagger/index.html`.

---

## Contributing

Build instructions, the dev loop, the test gates, the release process and the
architecture decision records all live in **[CONTRIBUTING.md](CONTRIBUTING.md)**.

## License

See [LICENSE](LICENSE).
