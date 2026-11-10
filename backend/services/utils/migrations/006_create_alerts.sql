-- +goose Up
CREATE TABLE market_mind.alerts (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(255) NOT NULL,
    subscription_id BIGINT NOT NULL REFERENCES market_mind.subscriptions(id) ON DELETE CASCADE,
    value NUMERIC NOT NULL,
    condition VARCHAR(3) NOT NULL,
    reference_price NUMERIC NOT NULL,
    is_notified BOOLEAN NOT NULL DEFAULT FALSE 
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX alerts_subscription_id_idx ON market_mind.alerts (subscription_id);

-- +goose Down
DROP TABLE market_mind.alerts;
