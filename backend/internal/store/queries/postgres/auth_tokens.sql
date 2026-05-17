-- name: CreateAuthToken :one
INSERT INTO auth_tokens (user_id, kind, token_hash, label, created_at, last_used_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, kind, token_hash, label, created_at, last_used_at, expires_at;

-- name: GetAuthTokenByHash :one
SELECT
    t.id,
    t.user_id,
    t.kind,
    t.token_hash,
    t.label,
    t.created_at,
    t.last_used_at,
    t.expires_at,
    u.username AS user_username,
    u.created_at AS user_created_at
FROM auth_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.token_hash = $1;

-- name: DeleteAuthTokenByHash :exec
DELETE FROM auth_tokens WHERE token_hash = $1;

-- name: DeleteAuthTokenByID :execrows
DELETE FROM auth_tokens
WHERE id = $1 AND user_id = $2 AND kind = 'api';

-- name: ListAPITokens :many
SELECT id, user_id, kind, token_hash, label, created_at, last_used_at, expires_at
FROM auth_tokens
WHERE user_id = $1 AND kind = 'api'
ORDER BY created_at DESC;

-- name: ListSessionTokens :many
SELECT id, user_id, kind, token_hash, label, created_at, last_used_at, expires_at
FROM auth_tokens
WHERE user_id = $1 AND kind = 'session'
ORDER BY created_at DESC;
