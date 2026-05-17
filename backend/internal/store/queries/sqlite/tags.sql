-- name: UpsertTag :one
-- Idempotent insert of a tag for (user_id, name). Returns the id either
-- way thanks to the UNIQUE(user_id, name) index.
INSERT INTO tag (user_id, name, created_at)
VALUES (?, ?, ?)
ON CONFLICT (user_id, name) DO UPDATE SET name = excluded.name
RETURNING id;

-- name: DeleteAllSeriesTagLinks :exec
-- Drops every series_tag row for the given series. Caller is
-- responsible for scoping by user; this runs inside a tx that has
-- already validated ownership.
DELETE FROM series_tag WHERE series_id = ?;

-- name: InsertSeriesTagLink :exec
INSERT INTO series_tag (series_id, tag_id) VALUES (?, ?)
ON CONFLICT DO NOTHING;

-- name: GetSeriesTags :many
SELECT t.name
FROM series_tag st
JOIN tag t ON t.id = st.tag_id
WHERE st.series_id = ?
ORDER BY t.name ASC;

-- name: ListTagsByUser :many
-- Test-only helper: lists every tag name owned by the given user in
-- sorted order. Used in the integration tests to assert store-state
-- after tag CRUD operations.
SELECT name FROM tag WHERE user_id = ? ORDER BY name ASC;

-- name: ListSeriesTagsBatch :many
SELECT st.series_id, t.name
FROM series_tag st
JOIN tag t ON t.id = st.tag_id
WHERE st.series_id IN (sqlc.slice('series_ids'))
ORDER BY st.series_id ASC, t.name ASC;
