-- +goose Up
CREATE SCHEMA IF NOT EXISTS market_mind;

CREATE TABLE market_mind.cryptocurrencies (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- +goose Down
DROP TABLE market_mind.cryptocurrencies;
DROP SCHEMA market_mind;
