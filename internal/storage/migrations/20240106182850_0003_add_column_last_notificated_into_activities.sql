-- +goose Up
-- +goose StatementBegin
ALTER TABLE activities ADD COLUMN last_pontificated Timestamp;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE activities DROP COLUMN last_pontificated;
-- +goose StatementEnd
