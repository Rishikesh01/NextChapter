# NextChapter

**Where was I?** A self-hosted progress tracker for manhwa, manhua, and web novels — the things you read across half a dozen scanlation and translation sites and lose your place in.

One Go binary serves the API and the web library; a browser extension captures your position as you read. Your data never leaves machines you own.

---

## The problem

Serialised reading is messy. A single series shows up under different titles, on different sites, in different translations. You start *Solo Leveling* on one aggregator, the chapter you want isn't there next week so you finish it on another, and now your bookmarks are split across three tabs in two browsers on two devices. Existing trackers (AniList, MAL) are catalogues, not progress tools, and they don't know what site you actually read on.

## What NextChapter does

- **Opt-in capture.** Click the extension button on a chapter page to record where you are. Nothing runs in the background unless you turn on auto-tracking for a specific site, one host at a time (ADR-0012).
- **Per-site tracking.** Every entry remembers both the *chapter number* and the *site you read it on*. Read *Solo Leveling* to ch 100 on Site A then continue to ch 110 on Site B, and NextChapter keeps both threads — it doesn't collapse them.
- **Manual series reassignment.** *The Beginning After The End* on one site is *TBATE* on another, possibly *오로지 너로 시작되는* on a third. Grab any entry and reassign it to a different (or new) series. Manual override is a first-class feature, not a debugging tool.
- **URL-pattern heuristics, with fallback.** For known sites the series slug and chapter number come out of the URL automatically (`/series/<slug>/chapter-<n>` and similar). For unknown sites you fill in the details, or build a rule without writing a regex.
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

Open <http://localhost:8080>, register an account, and you're running. Images are multi-arch (`linux/amd64`, `linux/arm64`) and run as a non-root user on a distroless base.

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

Skip `web-embed` and you get an API-only binary that answers `not_found` at `/` — fine for development, not what you want to deploy.

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

## Development

`make` at the repository root lists every target. Per-track detail lives in `backend/Makefile` and `frontend/Makefile`; the root Makefile is the entry point for anything spanning both.

### Prerequisites

Go 1.27+, Node 24 (`.nvmrc`), pnpm 10+, and Docker for the e2e gates and the Postgres tests.

```sh
make setup          # Go modules + pnpm workspace (frozen lockfile)
```

### The dev loop

Three terminals, or whichever two you need:

```sh
make dev-backend    # API on :8080, SQLite at backend/nextchapter.db
make dev-web        # SPA on :5173, Vite proxies API paths to :8080
make dev-extension  # extension in an auto-reloading Chromium
```

The Vite proxy puts the SPA and the API on one origin in dev exactly as they share one in production, so the session cookie works with no configuration.

### Checks

```sh
make lint           # gofmt + go vet + golangci-lint, eslint + prettier + tsc + depcheck
make test           # go test -race, and Vitest across the workspace
make test-e2e       # both Playwright suites in the pinned Docker image
```

`make test-e2e` is the QA gate CI runs: real Chromium, real backend binary, no mocked API. Playwright's browser download (~650 MB) is only needed for the *native* dev-loop variants (`make -C frontend test-e2e`) — `make setup-browsers` fetches it if you want them.

For the Postgres engine: `make -C backend pg-up` starts a local Postgres, `make -C backend test-postgres` runs the integration suite against a testcontainer.

### Codegen

Two generated surfaces, both checked for staleness in CI:

```sh
make -C backend sqlc        # SQL bindings → backend/internal/store/generated/
make -C backend swagger     # OpenAPI spec ← handler annotations
make -C frontend api-types  # TS types ← that spec → packages/api-client/src/generated/
```

The API contract flows one way: Go handler annotations → swagger → TypeScript. Change a handler, regenerate both, commit the output.

### Layout

```
backend/           Go API server — gin, sqlc, goose migrations (sqlite + postgres trees)
  internal/webui/  the embedded SPA build (a placeholder in git; ADR-0010)
frontend/          the MV3 extension — TypeScript, React 19, WXT
web/               the companion library SPA — React 19, Vite, react-router, react-query
packages/          api-client: the shared, generated-types API client
design/            design tokens, component specs, HTML prototypes
docs/adr/          architecture decision records — read these before changing shape
scripts/           repo tooling (disk-report)
```

### Stack

**Backend** — Go 1.27, `gin` for routing, `sqlc` for type-safe SQL, goose for migrations, `modernc.org/sqlite` (pure Go) by default with Postgres as a production option. `CGO_ENABLED=0` everywhere; pure-Go dependencies only. Tests run against real databases — testcontainers for Postgres, across a dual-engine CI matrix.

**Frontend** — TypeScript 5.9 strict, React 19, WXT, Manifest V3 for both Chrome and Firefox. Browser APIs through `wxt/browser`. pnpm workspace rooted at the repo. Vitest for units; Playwright against a real browser and a real backend binary for e2e (ADR-0008).

---

## Disk footprint

A full dev setup — both e2e Docker images, the Playwright browsers, the Go and pnpm caches — runs to tens of gigabytes, most of it caches shared with your other projects rather than anything inside the repo.

```sh
make disk-report    # what it costs right now, and which target reclaims each line
```

```sh
make clean          # build output in the working tree            (always safe)
make clean-deps     # …and node_modules                           (make setup-node restores)
make clean-docker   # NextChapter's images + the Docker build cache
make clean-caches   # the Go build cache; DEEP=1 adds the module cache + pnpm store
make clean-all      # all of the above
```

`clean-docker` removes only images named `nextchapter*` — every other image on your daemon is left alone. It does prune the whole Docker build cache, which is shared, but that cache is pure derived data and rebuilds on demand. Nothing any of these targets deletes is unrecoverable; the expensive ones cost a re-download or a rebuild.

The two e2e images are ~3.7 GB each and are rebuilt by every `make test-e2e` run, so they're the fastest-growing line. `make clean-docker` after a run of e2e work is the habit worth having.

---

## Releasing

One tag releases all three artefacts, stamped with the same version (ADR-0013):

```sh
git tag -a v1.2.3 -m 'v1.2.3'
git push origin v1.2.3
```

That's the entire process — no version file to bump, no release branch. `.github/workflows/release.yml` then lints, tests, and publishes to the GitHub Release for the tag:

| Artefact | What it's for |
| --- | --- |
| `nextchapter_v1.2.3_<os>_<arch>.tar.gz` / `.zip` | the server, SPA embedded — 6 platforms |
| `ghcr.io/rishikesh01/nextchapter:v1.2.3` | multi-arch image, plus `:latest` for stable tags |
| `nextchapter-extension-v1.2.3-chrome-mv3.zip` | Chromium build |
| `nextchapter-extension-v1.2.3-firefox-mv3.zip` | Firefox build (+ the sources zip AMO requires) |
| `nextchapter-web-v1.2.3.tar.gz` | the SPA alone, for fronting the API with nginx or a CDN |
| `checksums.txt` | sha256 over all of the above |

A tag containing `-` (`v1.2.3-rc.1`) is published as a prerelease and does not move `:latest`.

To build the same set locally — identical targets, no GitHub involved:

```sh
make dist VERSION=v1.2.3     # everything into dist/
make dist-backend            # or just one of them
```

Or rehearse the workflow itself: run it from the Actions tab via *workflow_dispatch*, which builds and uploads the full artefact set without publishing a release or pushing an image.

---

## Architecture decisions

The `docs/adr/` records are the reasoning behind the shape of this codebase — read the relevant one before changing an area.

| ADR | Subject |
| --- | --- |
| [0004](docs/adr/0004-web-ui-delivery.md) | Web UI delivery: JSON-only backend, separate SPA |
| [0007](docs/adr/0007-deferred-stdlib-cves.md) | Deferred stdlib CVEs and the re-enable condition |
| [0008](docs/adr/0008-extension-stack-and-testing.md) | Extension stack, permission model, e2e strategy |
| [0009](docs/adr/0009-extension-site-rule-management.md) | Site-rule management in the extension |
| [0010](docs/adr/0010-web-library-spa.md) | The web library SPA, cookie auth, single-binary embedding |
| [0011](docs/adr/0011-series-cover-images.md) | Series cover art (the extension fetches; the backend never does) |
| [0012](docs/adr/0012-auto-tracking.md) | Auto-tracking: per-host opt-in, advance-only capture |
| [0013](docs/adr/0013-release-flow-and-versioning.md) | Release flow: one tag, three artefacts |

## License

See [LICENSE](LICENSE).
