-- name: InsertSiteRule :one
INSERT INTO site_rule (
    user_id, host, chapter_url_regex,
    slug_capture_group, chapter_capture_group,
    created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, host, chapter_url_regex,
          slug_capture_group, chapter_capture_group,
          created_at, updated_at;

-- name: GetSiteRuleByID :one
SELECT id, user_id, host, chapter_url_regex,
       slug_capture_group, chapter_capture_group,
       created_at, updated_at
FROM site_rule
WHERE id = $1 AND user_id = $2;

-- name: GetSiteRuleByHost :one
SELECT id, user_id, host, chapter_url_regex,
       slug_capture_group, chapter_capture_group,
       created_at, updated_at
FROM site_rule
WHERE user_id = $1 AND host = $2;

-- name: ListSiteRulesByUser :many
SELECT id, user_id, host, chapter_url_regex,
       slug_capture_group, chapter_capture_group,
       created_at, updated_at
FROM site_rule
WHERE user_id = $1
ORDER BY host ASC;

-- name: UpdateSiteRule :one
UPDATE site_rule
SET host = $1,
    chapter_url_regex = $2,
    slug_capture_group = $3,
    chapter_capture_group = $4,
    updated_at = $5
WHERE id = $6 AND user_id = $7
RETURNING id, user_id, host, chapter_url_regex,
          slug_capture_group, chapter_capture_group,
          created_at, updated_at;

-- name: DeleteSiteRule :execrows
DELETE FROM site_rule WHERE id = $1 AND user_id = $2;

-- name: ListTrackedHosts :many
SELECT DISTINCT site_host
FROM entries
WHERE user_id = $1
ORDER BY site_host ASC;
