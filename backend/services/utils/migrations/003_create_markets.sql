-- +goose Up
CREATE TABLE market_mind.markets (
    id BIGSERIAL PRIMARY KEY,
    cryptocurrency_id BIGINT NOT NULL REFERENCES market_mind.cryptocurrencies(id) ON DELETE CASCADE,
    exchange_id INTEGER NOT NULL REFERENCES market_mind.exchanges(id) ON DELETE CASCADE,
    price NUMERIC NOT NULL CHECK (price > 0),
    UNIQUE (cryptocurrency_id, exchange_id)
);

CREATE INDEX markets_exchange_id_idx ON market_mind.markets (exchange_id);

-- +goose Down
DROP TABLE market_mind.markets;
