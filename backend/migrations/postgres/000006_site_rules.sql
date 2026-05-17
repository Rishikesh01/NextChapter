-- +goose Up
-- +goose StatementBegin
CREATE TABLE site_rule (
    id                    BIGSERIAL PRIMARY KEY,
    user_id               BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    host                  TEXT   NOT NULL,
    chapter_url_regex     TEXT   NOT NULL,
    slug_capture_group    TEXT   NOT NULL,
    chapter_capture_group TEXT   NOT NULL,
    created_at            TIMESTAMP NOT NULL,
    updated_at            TIMESTAMP NOT NULL,
    UNIQUE (user_id, host)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_site_rule_user ON site_rule(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_site_rule_user;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE site_rule;
-- +goose StatementEnd
