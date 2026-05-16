-- name: GetEntryByID :one
SELECT id, user_id, series_id, site_host, series_slug, site_title,
       last_chapter, last_url, last_captured_at, created_at, updated_at
FROM entries
WHERE id = ? AND user_id = ?;

-- name: GetEntryByKey :one
SELECT id, user_id, series_id, site_host, series_slug, site_title,
       last_chapter, last_url, last_captured_at, created_at, updated_at
FROM entries
WHERE user_id = ? AND site_host = ? AND series_slug = ?;

-- name: CreateEntry :one
INSERT INTO entries (
    user_id, series_id, site_host, series_slug, site_title,
    last_chapter, last_url, last_captured_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, user_id, series_id, site_host, series_slug, site_title,
          last_chapter, last_url, last_captured_at, created_at, updated_at;

-- name: AdvanceEntry :one
UPDATE entries
SET last_chapter = ?,
    last_url = ?,
    site_title = ?,
    last_captured_at = ?,
    updated_at = ?
WHERE id = ? AND user_id = ?
RETURNING id, user_id, series_id, site_host, series_slug, site_title,
          last_chapter, last_url, last_captured_at, created_at, updated_at;

-- name: UpdateEntry :one
UPDATE entries
SET series_id = ?,
    last_chapter = ?,
    last_url = ?,
    site_title = ?,
    updated_at = ?
WHERE id = ? AND user_id = ?
RETURNING id, user_id, series_id, site_host, series_slug, site_title,
          last_chapter, last_url, last_captured_at, created_at, updated_at;

-- name: ListEntriesAll :many
SELECT id, user_id, series_id, site_host, series_slug, site_title,
       last_chapter, last_url, last_captured_at, created_at, updated_at
FROM entries
WHERE user_id = ?
ORDER BY last_captured_at DESC
LIMIT ? OFFSET ?;

-- name: ListEntriesBySeries :many
SELECT id, user_id, series_id, site_host, series_slug, site_title,
       last_chapter, last_url, last_captured_at, created_at, updated_at
FROM entries
WHERE user_id = ? AND series_id = ?
ORDER BY last_captured_at DESC
LIMIT ? OFFSET ?;

-- name: ListEntriesAllForSeries :many
SELECT id, user_id, series_id, site_host, series_slug, site_title,
       last_chapter, last_url, last_captured_at, created_at, updated_at
FROM entries
WHERE user_id = ? AND series_id = ?
ORDER BY last_captured_at DESC;

-- name: CountEntriesAll :one
SELECT COUNT(*) FROM entries WHERE user_id = ?;

-- name: CountEntriesBySeries :one
SELECT COUNT(*) FROM entries WHERE user_id = ? AND series_id = ?;

-- name: DeleteEntry :execrows
DELETE FROM entries WHERE id = ? AND user_id = ?;

-- name: SeriesExists :one
SELECT EXISTS(SELECT 1 FROM series WHERE id = ? AND user_id = ?) AS found;
