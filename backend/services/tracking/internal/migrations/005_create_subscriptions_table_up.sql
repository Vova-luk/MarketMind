CREATE TABLE IF NOT EXISTS market_mind.subscriptions(
    id bigserial primary key,
    user_id bigint not null references market_mind.users(id) on delete cascade,
    market_id bigint not null references market_mind.markets(id) on delete cascade,
);