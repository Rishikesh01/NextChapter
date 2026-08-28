-- name: UpsertSeriesCover :one
-- series_id is the primary key, so ON CONFLICT replaces an existing cover
-- in place and created_at is preserved from the original row.
INSERT INTO series_cover (
    series_id, user_id, bytes, mime, byte_size, width, height, etag, source_url,
    created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (series_id) DO UPDATE SET
    bytes      = excluded.bytes,
    mime       = excluded.mime,
    byte_size  = excluded.byte_size,
    width      = excluded.width,
    height     = excluded.height,
    etag       = excluded.etag,
    source_url = excluded.source_url,
    updated_at = excluded.updated_at
RETURNING series_id, user_id, mime, byte_size, width, height, etag, source_url, created_at, updated_at;

-- name: GetSeriesCover :one
SELECT series_id, user_id, bytes, mime, byte_size, width, height, etag, source_url, created_at, updated_at
FROM series_cover
WHERE series_id = $1 AND user_id = $2;

-- name: DeleteSeriesCover :execrows
DELETE FROM series_cover WHERE series_id = $1 AND user_id = $2;
