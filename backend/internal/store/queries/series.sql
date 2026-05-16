-- name: CreateSeries :one
INSERT INTO series (user_id, title, status, rating, notes, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, user_id, title, status, rating, notes, created_at, updated_at;

-- name: GetSeriesByID :one
SELECT id, user_id, title, status, rating, notes, created_at, updated_at
FROM series
WHERE id = ? AND user_id = ?;

-- name: ListSeriesByStatus :many
SELECT
    s.id,
    s.user_id,
    s.title,
    s.status,
    s.rating,
    s.notes,
    s.created_at,
    s.updated_at,
    (SELECT MAX(e.last_chapter) FROM entries e WHERE e.series_id = s.id) AS highest_chapter,
    CAST((SELECT COUNT(*) FROM entries e WHERE e.series_id = s.id) AS INTEGER) AS entry_count,
    (SELECT MAX(e.last_captured_at) FROM entries e WHERE e.series_id = s.id) AS rollup_last_captured_at
FROM series s
WHERE s.user_id = ? AND s.status = ?
ORDER BY s.updated_at DESC
LIMIT ? OFFSET ?;

-- name: ListSeriesAll :many
SELECT
    s.id,
    s.user_id,
    s.title,
    s.status,
    s.rating,
    s.notes,
    s.created_at,
    s.updated_at,
    (SELECT MAX(e.last_chapter) FROM entries e WHERE e.series_id = s.id) AS highest_chapter,
    CAST((SELECT COUNT(*) FROM entries e WHERE e.series_id = s.id) AS INTEGER) AS entry_count,
    (SELECT MAX(e.last_captured_at) FROM entries e WHERE e.series_id = s.id) AS rollup_last_captured_at
FROM series s
WHERE s.user_id = ?
ORDER BY s.updated_at DESC
LIMIT ? OFFSET ?;

-- name: CountSeriesAll :one
SELECT COUNT(*) FROM series WHERE user_id = ?;

-- name: CountSeriesByStatus :one
SELECT COUNT(*) FROM series WHERE user_id = ? AND status = ?;

-- name: GetSeriesSummary :one
SELECT
    s.id,
    s.user_id,
    s.title,
    s.status,
    s.rating,
    s.notes,
    s.created_at,
    s.updated_at,
    (SELECT MAX(e.last_chapter) FROM entries e WHERE e.series_id = s.id) AS highest_chapter,
    CAST((SELECT COUNT(*) FROM entries e WHERE e.series_id = s.id) AS INTEGER) AS entry_count,
    (SELECT MAX(e.last_captured_at) FROM entries e WHERE e.series_id = s.id) AS rollup_last_captured_at
FROM series s
WHERE s.id = ? AND s.user_id = ?;

-- name: UpdateSeries :one
UPDATE series
SET title = ?,
    status = ?,
    rating = ?,
    notes = ?,
    updated_at = ?
WHERE id = ? AND user_id = ?
RETURNING id, user_id, title, status, rating, notes, created_at, updated_at;

-- name: DeleteSeries :execrows
DELETE FROM series WHERE id = ? AND user_id = ?;
