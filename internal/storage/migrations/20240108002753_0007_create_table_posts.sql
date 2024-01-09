-- +goose Up
-- +goose StatementBegin
CREATE TABLE posts (
    user_id Int64 NOT NULL,
    activity Text NOT NULL,
    ts Timestamp,
    PRIMARY KEY (user_id, activity, ts)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE posts;
-- +goose StatementEnd
