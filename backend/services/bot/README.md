# Bot Service

Бот читает `Price-Changed-Topic` в группе `marketmind-bot` и отправляет уведомления
через Telegram Bot API. Exchange Data публикует котировки в `Price-Checked-Topic`,
Tracking проверяет условия алертов и публикует готовые уведомления для бота.
Получатель берётся из поля `telegram_user_id` каждого уведомления.

```text
Сработал алерт!
Монета: BTC/USDT (Binance)
Цена при настройке алерта: 64000 USDT
Актуальная цена: 65000 USDT
Условие срабатывания: ≥ 65000 USDT
```

Для процентного алерта условие выглядит как `≥ +5%` или `≤ -5%`.
Цена на момент настройки алерта берётся из `reference_price`.

## Запуск в Docker Compose

Укажите `BOT_TOKEN` в `backend/.env` и запустите общий Compose с профилем
`notifications` из каталога `backend`. При первом запуске скопируйте `.env.example`
в `.env` перед настройкой токена.

```bash
docker compose --env-file .env -f deploy/compose.yaml --profile notifications up --build -d
docker compose --env-file .env -f deploy/compose.yaml --profile notifications logs -f bot
```

Для получения уведомлений в личном чате пользователь должен сначала начать диалог
с ботом, например нажать **Start**. [Документация Telegram](https://core.telegram.org/bots)

## Проверка доставки в Telegram

После запуска Compose отправьте своему боту в личном чате сообщение `marketmind-id`.
Из каталога `backend` получите ID отправителя. Команда использует `jq` и токен из
окружения контейнера.

```bash
docker compose --env-file .env -f deploy/compose.yaml --profile notifications exec -T bot \
  sh -c 'wget -qO- "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates"' |
  jq -r '.result[] | select(.message.chat.type == "private" and .message.text == "marketmind-id") | .message.from.id' |
  sort -u
```

Если у бота настроен webhook, возьмите `message.from.id` из полученного события.
[Telegram Bot API](https://core.telegram.org/bots/api#getupdates)

Подставьте свой ID и отправьте тестовый алерт в Kafka.

```bash
TELEGRAM_USER_ID=123456789
TEST_EVENT_ID="telegram-smoke-$(date +%s)"
docker compose --env-file .env -f deploy/compose.yaml exec -T broker-1 \
  /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server broker-1:19092 \
  --topic Price-Changed-Topic <<EOF
{"event_id":"$TEST_EVENT_ID","items":[{"notification_id":"$TEST_EVENT_ID","telegram_user_id":$TELEGRAM_USER_ID,"base_asset":"BTC","quote_asset":"USDT","exchange_name":"Binance","type":"percent","direction":"gte","value":"5","reference_price":"100","current_price":"106"}]}
EOF
```

В личном чате должно появиться сообщение «Сработал алерт!» с парой BTC/USDT.
Эта проверка охватывает путь от Kafka до Telegram Bot API.

## Локальный запуск

Запустите Kafka по [инструкции бэкенда](../../README.md#kafka-и-kafka-ui).

В каталоге `backend/services` скопируйте пример конфигурации.

```bash
cp bot/.env.example bot/.env
```

Укажите токен в `BOT_TOKEN` в файле `bot/.env` или в окружении процесса.

```bash
go run ./bot/cmd/bot -env-file bot/.env
curl http://localhost:8183/health
```

`GET /health` проверяет доступность HTTP-сервера. Ctrl+C или SIGTERM
останавливает сервер и обработку Kafka.

| Переменная | Назначение |
| --- | --- |
| `BOT_TOKEN` | Обязательный токен бота |
| `BOT_KAFKA_BROKERS` | Адреса брокеров через запятую. Для локального Compose в примере указаны `localhost:29092,localhost:39092` |
| `BOT_KAFKA_GROUP_ID` | Группа потребителей, по умолчанию `marketmind-bot` |
| `BOT_KAFKA_INPUT_TOPIC` | Входной топик, по умолчанию `Price-Changed-Topic` |
| `BOT_KAFKA_WORKERS` | Максимум одновременных отправок из разных партиций на экземпляр, по умолчанию `1` |
| `BOT_TELEGRAM_TIMEOUT` | Таймаут запроса к Telegram, по умолчанию `5s` |

Приоритет настроек от высшего к низшему — окружение процесса, указанный
`.env`, YAML, значения по умолчанию.

## Доставка

Бот обрабатывает записи каждой партиции последовательно и подтверждает offset
после ответа `ok: true` для каждого уведомления из записи. Бот сначала пытается
отправить все корректные уведомления, затем повторяет только неудачные отправки
с увеличивающейся задержкой и учитывает `retry_after` из Telegram.
При постоянной ошибке доставки или некорректном уведомлении следующие записи той
же партиции ждут, а остальные партиции продолжают обрабатываться. Сами записи
Kafka хранятся по настройкам retention.

Для параллельной обработки задайте, например, `BOT_KAFKA_WORKERS=3` или запустите
несколько экземпляров с одной группой. Число одновременно обрабатываемых записей
ограничено числом доступных партиций. Tracking использует ID рынка как ключ Kafka,
поэтому новые события одного рынка попадают в одну партицию.

Если бот перезапустится или потеряет партицию после отправки, но до фиксации offset,
запись может быть прочитана повторно и пользователь может получить дубликат.
