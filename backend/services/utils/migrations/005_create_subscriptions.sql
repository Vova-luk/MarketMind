-- +goose Up
CREATE TABLE market_mind.subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES market_mind.users(id) ON DELETE CASCADE,
    market_id BIGINT NOT NULL REFERENCES market_mind.markets(id) ON DELETE CASCADE,
    UNIQUE (user_id, market_id)
);

CREATE INDEX subscriptions_market_id_idx ON market_mind.subscriptions (market_id);

-- +goose Down
DROP TABLE market_mind.subscriptions;
