-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    user_id Int64 NOT NULL,
    hour_to_rotate_stats Int32,
    last_post_ts Timestamp,
    last_stats_rotate_ts Timestamp,
    PRIMARY KEY (user_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
