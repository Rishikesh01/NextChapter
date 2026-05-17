-- +goose Up
-- +goose StatementBegin
CREATE TABLE series (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'reading'
        CHECK (status IN ('reading','completed','on_hold','dropped','plan_to_read')),
    rating INTEGER CHECK (rating IS NULL OR (rating BETWEEN 1 AND 10)),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_series_user_status ON series(user_id, status);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_series_user_title ON series(user_id, title);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_series_user_title;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_series_user_status;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE series;
-- +goose StatementEnd
