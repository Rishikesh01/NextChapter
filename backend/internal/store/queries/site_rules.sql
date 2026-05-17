-- name: InsertSiteRule :one
INSERT INTO site_rule (
    user_id, host, chapter_url_regex,
    slug_capture_group, chapter_capture_group,
    created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, user_id, host, chapter_url_regex,
          slug_capture_group, chapter_capture_group,
          created_at, updated_at;

-- name: GetSiteRuleByID :one
SELECT id, user_id, host, chapter_url_regex,
       slug_capture_group, chapter_capture_group,
       created_at, updated_at
FROM site_rule
WHERE id = ? AND user_id = ?;

-- name: GetSiteRuleByHost :one
SELECT id, user_id, host, chapter_url_regex,
       slug_capture_group, chapter_capture_group,
       created_at, updated_at
FROM site_rule
WHERE user_id = ? AND host = ?;

-- name: ListSiteRulesByUser :many
SELECT id, user_id, host, chapter_url_regex,
       slug_capture_group, chapter_capture_group,
       created_at, updated_at
FROM site_rule
WHERE user_id = ?
ORDER BY host ASC;

-- name: UpdateSiteRule :one
UPDATE site_rule
SET host = ?,
    chapter_url_regex = ?,
    slug_capture_group = ?,
    chapter_capture_group = ?,
    updated_at = ?
WHERE id = ? AND user_id = ?
RETURNING id, user_id, host, chapter_url_regex,
          slug_capture_group, chapter_capture_group,
          created_at, updated_at;

-- name: DeleteSiteRule :execrows
DELETE FROM site_rule WHERE id = ? AND user_id = ?;

-- name: ListTrackedHosts :many
SELECT DISTINCT site_host
FROM entries
WHERE user_id = ?
ORDER BY site_host ASC;
