-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN registration_chat_id Int64;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN registration_chat_id;
-- +goose StatementEnd
