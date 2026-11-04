CREATE TABLE IF NOT EXISTS market_mind.markets(
    id bigserial primary key,
    cryptocurrency_id bigint not null references market_mind.cryptocurrency_id(id) on delete cascade,
    exchange_id int not null references market_mind.exchanges(id) on delete cascade,
    price integer not null,
);