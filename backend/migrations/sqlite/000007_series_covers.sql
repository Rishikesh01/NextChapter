-- +goose Up
-- +goose StatementBegin
-- One cover per series, in its own table rather than a column on `series`
-- so the hot list query never drags image bytes through (ADR-0011 §3).
-- series_id is the primary key: the relationship is 1:1 and an upsert on
-- conflict replaces the existing cover.
CREATE TABLE series_cover (
    series_id  INTEGER PRIMARY KEY REFERENCES series(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bytes      BLOB    NOT NULL,
    mime       TEXT    NOT NULL CHECK (mime IN ('image/jpeg','image/png','image/webp')),
    byte_size  INTEGER NOT NULL CHECK (byte_size > 0),
    width      INTEGER NOT NULL CHECK (width > 0),
    height     INTEGER NOT NULL CHECK (height > 0),
    etag       TEXT    NOT NULL,
    source_url TEXT    NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_series_cover_user ON series_cover(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_series_cover_user;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE series_cover;
-- +goose StatementEnd
