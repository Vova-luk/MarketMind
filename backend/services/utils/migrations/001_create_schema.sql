-- +goose Up
CREATE SCHEMA IF NOT EXISTS market_mind;

-- +goose Down
DROP SCHEMA market_mind;
