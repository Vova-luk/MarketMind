-- +goose Up
CREATE TABLE market_mind.users (
    id BIGSERIAL PRIMARY KEY,
    telegram_user_id BIGINT NOT NULL UNIQUE,
    is_notification_enable BOOLEAN NOT NULL DEFAULT TRUE
);

-- +goose Down
DROP TABLE market_mind.users;
