-- +goose Up
-- +goose StatementBegin
CREATE TABLE activities (
    user_id Int64 NOT NULL,
    activity Text NOT NULL,
    total Uint64,
    current Uint64,
    PRIMARY KEY (user_id, activity)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE activities;
-- +goose StatementEnd
