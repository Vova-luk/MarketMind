CREATE TABLE IF NOT EXISTS market_mind.exchanges(
    id integer primary key,
    name varchar(255) unique not null,
    is_active boolean not null default TRUE,
);