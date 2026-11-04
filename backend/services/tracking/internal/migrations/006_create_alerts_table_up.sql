CREATE TABLE IF NOT EXISTS market_mind.alerts(
    id bigserial primary key,
    type varchar(255),
    subscription_id bigint not null references market_mind.subscriptions(id) on delete cascade,
    value integer not null,
    reference_price integer not null,
    is_active boolean not null default TRUE,
);