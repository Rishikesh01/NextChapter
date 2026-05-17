-- name: CreateSeries :one
INSERT INTO series (user_id, title, status, rating, notes, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, title, status, rating, notes, created_at, updated_at;

-- name: GetSeriesByID :one
SELECT id, user_id, title, status, rating, notes, created_at, updated_at
FROM series
WHERE id = $1 AND user_id = $2;

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
    (SELECT COUNT(*) FROM entries e WHERE e.series_id = s.id)::bigint AS entry_count,
    (SELECT MAX(e.last_captured_at) FROM entries e WHERE e.series_id = s.id) AS rollup_last_captured_at
FROM series s
WHERE s.user_id = $1 AND s.status = $2
ORDER BY s.updated_at DESC
LIMIT sqlc.arg(lim)::bigint OFFSET sqlc.arg(off)::bigint;

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
    (SELECT COUNT(*) FROM entries e WHERE e.series_id = s.id)::bigint AS entry_count,
    (SELECT MAX(e.last_captured_at) FROM entries e WHERE e.series_id = s.id) AS rollup_last_captured_at
FROM series s
WHERE s.user_id = $1
ORDER BY s.updated_at DESC
LIMIT sqlc.arg(lim)::bigint OFFSET sqlc.arg(off)::bigint;

-- name: CountSeriesAll :one
SELECT COUNT(*) FROM series WHERE user_id = $1;

-- name: CountSeriesByStatus :one
SELECT COUNT(*) FROM series WHERE user_id = $1 AND status = $2;

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
    (SELECT COUNT(*) FROM entries e WHERE e.series_id = s.id)::bigint AS entry_count,
    (SELECT MAX(e.last_captured_at) FROM entries e WHERE e.series_id = s.id) AS rollup_last_captured_at
FROM series s
WHERE s.id = $1 AND s.user_id = $2;

-- name: UpdateSeries :one
UPDATE series
SET title = $1,
    status = $2,
    rating = $3,
    notes = $4,
    updated_at = $5
WHERE id = $6 AND user_id = $7
RETURNING id, user_id, title, status, rating, notes, created_at, updated_at;

-- name: DeleteSeries :execrows
DELETE FROM series WHERE id = $1 AND user_id = $2;
