# 0007 - Deferred stdlib CVEs and govulncheck in CI

## Context

The project pins the Go toolchain to `1.26.2` (see `go.mod` and
`.github/workflows/ci.yml`). Running `govulncheck ./...` against that
toolchain reports three reachable **standard-library** CVEs. All three
are fixed by the toolchain patch release `go1.26.3`; none originates in
NextChapter's own code or in a third-party module we could bump
independently.

That leaves a choice for CI: run `govulncheck` as a blocking step now —
in which case CI is red until the host runners ship `go1.26.3` (or we
flip `GOTOOLCHAIN=auto`) — or defer the gate and track the finding out
of band until the toolchain moves.

## Decision

**Defer `govulncheck` as a CI gate; keep it as a manual target.**

- `govulncheck` is **not** run in the CI workflow yet. The `lint` job
  runs gofmt + `go vet` + golangci-lint only; the CI comment documents
  why the vuln scan is absent.
- The scan stays available on demand as `make security-audit`
  (`govulncheck ./...`), so an operator or reviewer can inspect the
  deferred findings at any time.
- The only outstanding findings are the three stdlib CVEs above, all
  resolved by a toolchain bump to `go1.26.3`. No application-code or
  dependency change is required.

### 2026-08-28 refresh (feat/web-bootstrap)

The deferred set has grown with new advisories: the scan now reports
**13 reachable stdlib CVEs**, with fixes spanning `go1.26.3`–`go1.26.6`
(html/template, crypto/tls, net/http, encoding/xml, net/mail,
net/textproto, net, encoding/asn1, crypto/x509). All remain
toolchain-patch-only — the deferral mechanism is unchanged, but the
re-enable target is now `go1.26.6`.

Two findings that were NOT toolchain-only — `golang.org/x/text`
(GO-2026-5970) and `github.com/quic-go/quic-go` (GO-2026-5676), both
indirect modules — fell outside this deferral and were fixed by
dependency bumps on this branch, per the audit gate. The original
"none originates in a bumpable module" premise should be re-checked on
every audit run, not assumed.

## Consequences

- CI does not fail on a finding we cannot fix without a toolchain patch
  release, avoiding a permanently-red "documented red" gate.
- The trade-off is that a *newly introduced* vulnerability (e.g. via a
  future dependency) would not be caught automatically until the CI gate
  is re-enabled. `make security-audit` is the interim manual backstop.
- **Re-enable the CI gate** once the runners have `go1.26.6` (or we set
  `GOTOOLCHAIN=auto`) and the deferred stdlib findings clear (13 as of
  the 2026-08-28 refresh above). At that point `govulncheck` should move
  into the `backend` job alongside `make lint`.
