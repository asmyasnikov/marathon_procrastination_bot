-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN last_activity_ts Timestamp;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN last_activity_ts;
-- +goose StatementEnd
