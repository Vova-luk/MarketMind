-- +goose Up
CREATE TABLE market_mind.exchanges (
    id INTEGER PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

-- +goose Down
DROP TABLE market_mind.exchanges;
