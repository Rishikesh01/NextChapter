-- +goose Up
-- +goose StatementBegin
CREATE TABLE entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    series_id INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    site_host TEXT NOT NULL,
    series_slug TEXT NOT NULL,
    site_title TEXT NOT NULL,
    last_chapter REAL NOT NULL CHECK (last_chapter >= 0),
    last_url TEXT NOT NULL,
    last_captured_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    UNIQUE (user_id, site_host, series_slug)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_entries_series_id ON entries(series_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_entries_user_captured ON entries(user_id, last_captured_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_entries_user_captured;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_entries_series_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE entries;
-- +goose StatementEnd
