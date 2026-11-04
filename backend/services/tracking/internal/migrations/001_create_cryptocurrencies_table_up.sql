CREATE SCHEMA IF NOT EXISTS market_mind;

CREATE TABLE IF NOT EXISTS market_mind.cryptocurrencies(
    id bigserial primary key,
    name varchar(255) not null,
);