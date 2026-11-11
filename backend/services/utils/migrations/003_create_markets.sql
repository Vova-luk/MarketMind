-- +goose Up
CREATE TABLE market_mind.markets (
    id BIGSERIAL PRIMARY KEY,
    exchange_id INTEGER NOT NULL REFERENCES market_mind.exchanges(id) ON DELETE CASCADE,
    external_id VARCHAR(255) NOT NULL,
    base_asset VARCHAR(32) NOT NULL,
    quote_asset VARCHAR(32) NOT NULL,
    price NUMERIC NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT statement_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT statement_timestamp(),
    UNIQUE (exchange_id, external_id),
    UNIQUE (exchange_id, base_asset, quote_asset)
);

CREATE INDEX markets_exchange_id_idx ON market_mind.markets (exchange_id);

-- +goose Down
DROP TABLE market_mind.markets;
