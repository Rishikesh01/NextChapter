-- +goose Up
-- +goose StatementBegin
CREATE TABLE tag (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    created_at TIMESTAMP NOT NULL,
    UNIQUE (user_id, name)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_tag_user ON tag(user_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE series_tag (
    series_id INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    tag_id    INTEGER NOT NULL REFERENCES tag(id)    ON DELETE CASCADE,
    PRIMARY KEY (series_id, tag_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_series_tag_tag ON series_tag(tag_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_series_tag_tag;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE series_tag;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tag_user;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE tag;
-- +goose StatementEnd
