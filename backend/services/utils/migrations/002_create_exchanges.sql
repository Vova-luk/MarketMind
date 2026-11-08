-- +goose Up
CREATE TABLE market_mind.exchanges (
    id INTEGER PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO market_mind.exchanges (id, name, is_active)
VALUES
    (1, 'Binance', TRUE),
    (2, 'Bybit', TRUE),
    (3, 'OKX', TRUE);

-- +goose Down
DROP TABLE market_mind.exchanges;
