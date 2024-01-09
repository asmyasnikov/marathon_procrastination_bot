-- +goose Up
-- +goose StatementBegin
ALTER TABLE activities ADD COLUMN last_notificated Timestamp;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE activities DROP COLUMN last_notificated;
-- +goose StatementEnd
