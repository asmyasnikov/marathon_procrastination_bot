-- +goose Up
-- +goose StatementBegin
UPDATE activities SET last_notificated=last_pontificated;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE activities SET last_pontificated=last_notificated;
-- +goose StatementEnd
