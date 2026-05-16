-- name: CreateAuthToken :one
INSERT INTO auth_tokens (user_id, kind, token_hash, label, created_at, last_used_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
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
    u.password_hash AS user_password_hash,
    u.created_at AS user_created_at,
    u.updated_at AS user_updated_at
FROM auth_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.token_hash = ?;

-- name: DeleteAuthTokenByHash :exec
DELETE FROM auth_tokens WHERE token_hash = ?;

-- name: DeleteAuthTokenByID :execrows
DELETE FROM auth_tokens
WHERE id = ? AND user_id = ? AND kind = 'api';

-- name: ListAPITokens :many
SELECT id, user_id, kind, token_hash, label, created_at, last_used_at, expires_at
FROM auth_tokens
WHERE user_id = ? AND kind = 'api'
ORDER BY created_at DESC;

-- name: ListSessionTokens :many
SELECT id, user_id, kind, token_hash, label, created_at, last_used_at, expires_at
FROM auth_tokens
WHERE user_id = ? AND kind = 'session'
ORDER BY created_at DESC;
