CREATE TABLE IF NOT EXISTS market_mind.users(
    id bigserial primary key,
    telegram_user_id bigint not null,
    is_notification_enable boolean not null default TRUE,
);