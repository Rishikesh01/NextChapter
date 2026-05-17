-- +goose Up
-- +goose StatementBegin
CREATE TABLE auth_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('session','api')),
    token_hash TEXT NOT NULL UNIQUE,
    label TEXT,
    created_at TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_auth_tokens_token_hash ON auth_tokens(token_hash);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_auth_tokens_user_kind ON auth_tokens(user_id, kind);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_auth_tokens_user_kind;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_auth_tokens_token_hash;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE auth_tokens;
-- +goose StatementEnd
