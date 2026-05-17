-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CreateUser :one
INSERT INTO users (username, password_hash, created_at, updated_at)
VALUES (?, ?, ?, ?)
RETURNING id, username, password_hash, created_at, updated_at;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, created_at, updated_at
FROM users
WHERE username = ?;
