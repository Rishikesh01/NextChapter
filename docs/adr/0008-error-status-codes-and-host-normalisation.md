# 0008 - Bind-failure status codes and site_host normalisation

## Context

Two issues surfaced in the bootstrap reviewer pass that are worth
persisting:

1. **400 vs 422 on bind failure.** `ShouldBindJSON` can fail in two
   distinct ways: the request body is not valid JSON at all (or has a
   type mismatch the decoder can't recover from), or the JSON parsed
   but a field violated a `binding:"..."` rule. The original
   handlers were inconsistent: most `POST`/`PATCH` paths returned 400
   on parse failure (via `render.BadRequest`); `PATCH /entries/{id}`
   returned 422 (via `render.ValidationError`). The OpenAPI contract
   only declared 422 for the request-body endpoints, so 400 was an
   undocumented response.

2. **`site_host` normalisation.** ADR-0005 states that the persisted
   `site_host` on `entries` is "lowercased, no `www.`" and that the
   unique key `(user_id, site_host, series_slug)` is the
   deduplication identity. The original capture handler did
   `strings.ToLower(strings.TrimPrefix(req.SiteHost, "www."))`, which
   is a no-op against `"WWW.Foo.com"` because `TrimPrefix` is
   case-sensitive — the lowercase form runs *after* the prefix check,
   leaving the `www.` in place. The result: `"WWW.Foo.com"` persists
   as `"www.foo.com"`, while a subsequent capture against
   `"foo.com"` allocates a *new* entry row instead of advancing the
   same one. The capture model's deduplication promise is silently
   broken whenever the extension sends a mixed-case host.

## Decision

### Bind-failure semantics

We keep the two status codes distinct. They mean different things and
both are useful to the client:

- **400 Bad Request** — the request body could not be parsed.
  Malformed JSON, wrong content type, structural type mismatch (e.g.
  string where the schema expects a number). The `error.fields` map
  is omitted because no field-level information is available; the
  parser failed before binding completed.
- **422 Unprocessable Entity** — the request body parsed cleanly but
  one or more field values failed validation (out of range, missing
  required field, unknown enum, length bound, etc.). The
  `error.fields` map carries per-field messages.

A single helper, `bindJSON(c, &req)` in
`internal/httpapi/handlers/validation.go`, is the only call site of
`c.ShouldBindJSON` across the handler layer. It introspects the bind
error via `errors.As(err, &validator.ValidationErrors)`: a validator
error becomes 422 with `error.fields`, anything else (JSON syntax,
type mismatch, EOF, unknown content type) becomes 400 with no
`fields`. Every JSON-body handler routes through this helper, so the
400/422 split is uniform.

An earlier revision of this ADR carved out `PATCH /entries/{id}` as
"422-only" on the rationale that its field validation lived entirely
in `binding` tags and was therefore indistinguishable from the bind
step. That carve-out was abandoned when the helper landed: the
helper distinguishes the two cases by error type rather than by
handler structure, so `PATCH /entries/{id}` now emits 400 on a
malformed body just like every other JSON-body endpoint. The
OpenAPI spec reflects this — `paths./entries/{id}.patch` declares
both 400 and 422.

`docs/api/openapi.yaml` declares 400 (`#/components/responses/BadRequest`)
on every endpoint with a JSON request body, no exceptions.

### `site_host` normalisation

The capture handler normalises in this order: **lowercase first,
strip `www.` second.**

```go
host := strings.TrimPrefix(strings.ToLower(req.SiteHost), "www.")
```

Any future code that touches `site_host` going onto the wire or into
the DB must follow the same order. The rule lives next to the call
site in `entries.go`; no helper is introduced for it because there
is exactly one capture path.

We do **not** normalise the slug. Slugs are opaque per ADR-0005;
case differences in slugs represent different URL paths and would in
fact be distinct on the source site. The site-side case sensitivity
is the site's problem; the extension sends what the URL had.

## Consequences

- Clients can distinguish "your JSON is broken" from "your data is
  wrong" without parsing the error message.
- `error.fields` is meaningful: it's only present when the server
  knows which fields are wrong.
- Captures from mixed-case hosts (`WWW.reader.example.com`,
  `reader.example.com`) collapse onto the same `(user_id, site_host,
  series_slug)` key as their canonical lowercased form, preserving
  ADR-0005's deduplication contract.
- The integration suite covers the host-normalisation rule:
  `TestCaptureNormalisesSiteHost` issues a capture with a mixed-case
  host (`"WWW.reader.example.com"`) and asserts both that the persisted
  `site_host` column is lowercased without `www.`, and that a
  follow-up capture using the canonical lowercase form advances the
  same entry id rather than allocating a new row (verified via
  `CountEntriesBySeries == 1`).
