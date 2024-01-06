-- +goose Up
-- +goose StatementBegin
ALTER TABLE activities ADD COLUMN post_ts Timestamp;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE activities DROP COLUMN post_ts;
-- +goose StatementEnd
