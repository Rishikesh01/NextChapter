# 0013 - Release flow: one tag, three artefacts

## Context

NextChapter ships three things a user installs — the server, the browser extension, and the web
library SPA — and until now shipped none of them. There were no tags, no release workflow, and
no way to build a distributable set of artefacts other than reading `ci.yml` and reproducing its
steps by hand. `config.Version` existed and was surfaced from `/healthz`, but nothing ever set
it: every binary ever built reported `dev`.

The three artefacts are not independent. ADR-0010 embeds the SPA build into the Go binary, so
"the server" and "the SPA" are the same download in the normal case. The extension talks to a
server over the API contract that `packages/api-client` generates from that same server's
swagger. Releasing them on separate cadences would mean maintaining a compatibility matrix
between components that are developed, tested, and reviewed as one tree.

Two further constraints shaped this. The project has no root Makefile: `backend/Makefile` and
`frontend/Makefile` each own their track, and neither can own an artefact that spans both. And
CI is required to call `make` targets rather than reimplement build commands, so whatever the
release does must be expressible as targets a developer can also run locally.

## Decisions

1. **The git tag is the only version.** `VERSION` is `git describe --tags --always --dirty`,
   which yields the tag on a tagged commit, `<sha>[-dirty]` otherwise. There is no version
   constant to bump, no `package.json` field to edit, and no release commit. `package.json`
   versions stay at their placeholder and are never the source of truth — a tag is cut, and
   every artefact in that run carries its name.

2. **One tag releases all three.** `v1.2.3` produces the server binaries, both extension zips,
   and the SPA tarball, all stamped `v1.2.3`. Component versions cannot drift because there are
   no component versions. The cost — a release for a change touching one track bumps all three —
   is accepted: it is cheaper than a compatibility matrix.

3. **A repository-root Makefile owns everything cross-track.** `make dist` builds every
   artefact; `make setup`/`make dev-*` is the dev loop; `make disk-report`/`make clean-*` is
   disk hygiene. It delegates to the two per-track Makefiles rather than duplicating them, and
   `release.yml` calls its targets — the same commands a developer runs locally, so a release
   cannot break in a way local iteration would not reproduce.

4. **Version reaches the binary through the linker, not the environment.**
   `config.defaultVersion` is a package-level `var` stamped with
   `-ldflags -X`, and `Default()` returns it. The `NEXTCHAPTER_VERSION` env override is
   retained and still wins at runtime; an unstamped build (`go run`, `go test`, a bare
   `go build`) still reports `dev`. The Dockerfile takes the same value as a `VERSION` build
   arg. `/healthz` therefore tells an operator exactly which release they are running, which
   nothing before this could.

5. **The extension manifest carries a normalised version.** Chrome and Firefox accept only 1-4
   dot-separated integers, so `wxt.config.ts` strips the tag's `v` prefix and any
   prerelease/build metadata for `version`, and keeps the full tag in `version_name` — a
   Chromium-only key, since AMO's linter rejects it. A tag that does not reduce to a valid
   version falls back to `package.json`, so `make dist-extension` works on an untagged tree.

6. **Release artefacts, and what each is for.** Server archives per platform
   (`linux/{amd64,arm64,arm}`, `darwin/{amd64,arm64}`, `windows/amd64` — arm is GOARM=7, for a
   Pi) with the SPA embedded; a multi-arch `linux/{amd64,arm64,arm/v7}` image on GHCR,
   carrying OCI labels so `docker inspect` reports its version and revision and so
   GHCR links the package to this repository; both extension
   zips plus the sources zip AMO requires; and the SPA alone as a tarball, for the operator who
   fronts the API with nginx or a CDN rather than serving the UI from the binary. Plus
   `checksums.txt` over all of it.

7. **Store submission is deliberately out.** The extension ships as zips on the GitHub Release.
   Automated Chrome Web Store and AMO submission needs store accounts, review turnaround, and
   long-lived publishing secrets in CI — none of which a self-hosted tool needs to be
   installable, and all of which are a supply-chain surface. Sideloading is the documented path;
   the sources zip means AMO submission stays possible by hand.

8. **The release re-verifies.** `release.yml` runs lint and unit tests for both tracks before
   building anything. It does not repeat the Docker e2e gates — `ci.yml` already ran those on
   the commit being tagged — so a tag costs minutes, not the full matrix.

9. **`workflow_dispatch` is a rehearsal, not a release.** It builds and uploads the identical
   artefact set as workflow artifacts, but creates no GitHub Release and pushes no image. The
   release path can be exercised without spending a version number.

## Consequences

- Cutting a release is `git tag -a vX.Y.Z && git push origin vX.Y.Z`. Nothing else. There is no
  branch to prepare and no file to edit, so a release cannot be half-done.
- `dist-backend` and the Docker targets embed the SPA and must restore the committed placeholder
  afterwards (ADR-0010 §3). They do it from a shell `trap`, so a failed cross-compile cannot
  leave a real build sitting in the embed directory pretending to be tracked content.
- A prerelease tag (`v1.2.3-rc.1`) is published as a GitHub prerelease and never moves the
  image's `:latest`. Its extension manifest reads `1.2.3` with `version_name: v1.2.3-rc.1`,
  which means an rc and its final release are indistinguishable to Chromium's update check —
  acceptable while installs are unpacked, and a reason not to ship rcs to a store later.
- The `nextchapter_<version>_<os>_<arch>` archives are ~14 MB each and there are six of them, so
  each release adds ~85 MB to the repository's release storage.
- Deferred: signed artefacts and an SBOM (both matter more once installs are not manual);
  reproducible-build verification; and store submission per §7.
