-- name: GetEntryByID :one
SELECT id, user_id, series_id, site_host, series_slug, site_title,
       last_chapter, last_url, last_captured_at, created_at, updated_at
FROM entries
WHERE id = $1 AND user_id = $2;

-- name: GetEntryByKey :one
SELECT id, user_id, series_id, site_host, series_slug, site_title,
       last_chapter, last_url, last_captured_at, created_at, updated_at
FROM entries
WHERE user_id = $1 AND site_host = $2 AND series_slug = $3;

-- name: CreateEntry :one
INSERT INTO entries (
    user_id, series_id, site_host, series_slug, site_title,
    last_chapter, last_url, last_captured_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, user_id, series_id, site_host, series_slug, site_title,
          last_chapter, last_url, last_captured_at, created_at, updated_at;

-- name: AdvanceEntry :one
UPDATE entries
SET last_chapter = $1,
    last_url = $2,
    site_title = $3,
    last_captured_at = $4,
    updated_at = $5
WHERE id = $6 AND user_id = $7
RETURNING id, user_id, series_id, site_host, series_slug, site_title,
          last_chapter, last_url, last_captured_at, created_at, updated_at;

-- name: UpdateEntry :one
UPDATE entries
SET series_id = $1,
    last_chapter = $2,
    last_url = $3,
    site_title = $4,
    updated_at = $5
WHERE id = $6 AND user_id = $7
RETURNING id, user_id, series_id, site_host, series_slug, site_title,
          last_chapter, last_url, last_captured_at, created_at, updated_at;

-- name: ListEntriesAll :many
SELECT id, user_id, series_id, site_host, series_slug, site_title,
       last_chapter, last_url, last_captured_at, created_at, updated_at
FROM entries
WHERE user_id = $1
ORDER BY last_captured_at DESC
LIMIT sqlc.arg(lim)::bigint OFFSET sqlc.arg(off)::bigint;

-- name: ListEntriesBySeries :many
SELECT id, user_id, series_id, site_host, series_slug, site_title,
       last_chapter, last_url, last_captured_at, created_at, updated_at
FROM entries
WHERE user_id = $1 AND series_id = $2
ORDER BY last_captured_at DESC
LIMIT sqlc.arg(lim)::bigint OFFSET sqlc.arg(off)::bigint;

-- name: ListEntriesAllForSeries :many
SELECT id, user_id, series_id, site_host, series_slug, site_title,
       last_chapter, last_url, last_captured_at, created_at, updated_at
FROM entries
WHERE user_id = $1 AND series_id = $2
ORDER BY last_captured_at DESC;

-- name: CountEntriesAll :one
SELECT COUNT(*) FROM entries WHERE user_id = $1;

-- name: CountEntriesBySeries :one
SELECT COUNT(*) FROM entries WHERE user_id = $1 AND series_id = $2;

-- name: DeleteEntry :execrows
DELETE FROM entries WHERE id = $1 AND user_id = $2;

-- name: SeriesExists :one
-- Postgres EXISTS returns boolean and cannot be directly cast to
-- bigint; the CASE wrapper yields 0/1 as an integer literal that
-- sqlc reads as bigint. This matches the SQLite variant (which returns
-- int64) so the repository can stay engine-agnostic. The repository
-- treats non-zero as true.
SELECT (CASE WHEN EXISTS(SELECT 1 FROM series WHERE id = $1 AND user_id = $2) THEN 1 ELSE 0 END)::bigint AS found;
