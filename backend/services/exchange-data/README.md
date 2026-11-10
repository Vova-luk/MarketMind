# Exchange-Data-Service

Сервис запускает HTTP-сервер состояния на порту `8184`, gRPC-поиск на порту `9090`
и планировщик сбора цен.
Первый сбор начинается сразу, каждый следующий — через `scheduler.interval`
после завершения предыдущего. Циклы не пересекаются; ошибки цикла логируются,
а последующие проверки продолжаются. `/health` подтверждает работу HTTP-сервера,
но не доступность бирж или Kafka.

Пока список рынков возвращает `StubMarketSource`: BTCUSDT на Binance и Bybit,
BTC-USDT на OKX. Market ID 42/43/44 — учебные, поэтому такой запуск предназначен
для разработки, а не для отправки в рабочий tracking с настоящими подписками.

Из `backend/services`:

```bash
go run ./exchange-data/cmd/exchange-data
```

Для локальных переопределений:

```bash
cp exchange-data/.env.example exchange-data/.env
go run ./exchange-data/cmd/exchange-data -config exchange-data/config/config.yaml -env-file exchange-data/.env
```

Приоритет настроек: окружение процесса > явно переданный `.env` > YAML > defaults.
Конфиг заполняется напрямую в структуры `internal/config`; `.env` читает godotenv.

```bash
curl http://localhost:8184/health
go test -race ./exchange-data/...
```

## Структура

- `cmd/exchange-data` — точка входа, конфиг, slog и обработка сигналов.
- `internal/app` — сборка компонентов и запуск приложения.
- `internal/config` — структуры настроек, YAML, окружение и валидация.
- `internal/server` — HTTP-сервер состояния, heartbeat и остановка.
- `internal/domain` — место для общих моделей рынков и котировок.
- `internal/service` — место для сценариев приложения и интерфейсов зависимостей.
- `internal/adapters/inbound/grpc` — место для gRPC-запросов от API-Service.
- `internal/adapters/inbound/scheduler` — место для периодического обновления котировок.
- `internal/adapters/outbound/crypto_exchanges` — места для клиентов Binance, Bybit и OKX.
- `internal/adapters/outbound/kafka_producer` — место для публикации в `Price-Checked-Topic`.

## Поиск рынков по gRPC

Метод `SearchMarkets` принимает `exchange_name`, `query`, `page` и `limit`.
Пустая строка соответствует всему активному спотовому каталогу выбранной биржи.
Страница по умолчанию `1`, лимит `20`, максимум `100`. Совпадения сортируются
по `external_id` до пагинации. `next_page` присутствует, если есть ещё результаты.

Выполните команду из `backend/services`.

```bash
grpcurl -plaintext -import-path proto -proto exchange-data/search.proto \
  -d '{"exchange_name":"Binance","query":"btc","page":1,"limit":5}' \
  127.0.0.1:9090 marketmind.exchangedata.v1.MarketSearchService/SearchMarkets
```

Метаданные пар кэшируются на минуту, последние цены на пять секунд. Поиск
использует публичные данные бирж. Чтобы запустить только HTTP и gRPC, задайте
`EXCHANGE_DATA_SCHEDULER_DISABLED=true`. Compose запускает Exchange-Data в этом режиме.

По Ctrl+C или SIGTERM общий контекст отменяется: прекращаются запросы и ожидания,
завершаются планировщик, HTTP и gRPC, затем закрывается Kafka writer.
Пока эти компоненты не остановились, producer не закрывается.

## Настройки Binance

`exchanges.binance` задаёт `base_url`, `request_timeout`, `retry_delay` и
`max_attempts`. Настройки можно переопределить через
`EXCHANGE_DATA_BINANCE_BASE_URL`, `EXCHANGE_DATA_BINANCE_TIMEOUT`,
`EXCHANGE_DATA_BINANCE_RETRY_DELAY`, `EXCHANGE_DATA_BINANCE_MAX_ATTEMPTS`.
`max_attempts` включает первую попытку; значение 1 означает отсутствие повторов.

`app` создаёт Binance-клиент из загруженного конфига. Адрес и таймаут уже
используются при запросах. Клиент повторяет таймауты отдельных HTTP-попыток и ответы 502/503/504,
ожидая `retry_delay` между попытками. После последней попытки ожидания нет.
Общий `context` прерывает и запросы, и ожидание. Остальные HTTP-статусы
(включая 429), ошибки декодирования и некорректные котировки не повторяются.
Сервисный слой и автоматический запуск опроса подключены в `app`.

## REST-клиент OKX

`okx.NewOKXClient(cfg.Exchanges.OKX, logger)` создаётся в `app`.
`GetPrices(ctx, symbols)` получает спотовые тикеры через
`GET /api/v5/market/tickers?instType=SPOT` и выбирает запрошенные пары.
Передавайте символы в формате OKX, например `BTC-USDT` и `ETH-USDT`;
автоматического преобразования `BTCUSDT` нет.

Результат содержит по одному элементу на уникальный непустой символ в порядке
запроса. Отсутствующие пары и некорректные цены отмечаются в `Cryptocurrency.Error`.
Пустые строки удаляются на месте: исходный массив аргумента изменяется.
Ценой служит `last` — цена последней сделки; исходная десятичная строка сохраняется.

Таймаут, задержка и число попыток берутся из `exchanges.okx`.
Повторяются таймауты и HTTP 502/503/504. HTTP 429 и ошибки API в поле `code`
возвращаются без повторов. `Warn` сообщает о повторе, `Debug` — об итогах запроса.
Планировщик и gRPC-поиск используют этот REST-клиент через сервисный слой.

[Документация OKX: GET / Tickers](https://www.okx.com/docs-v5/en/#order-book-trading-market-data-get-tickers).

## Общие HTTP-ретраи

Binance, Bybit и OKX используют `internal/utils.DoWithRetry`.
Функция получает HTTP-клиент, запрос, настройки попыток и логгер.
Её сообщения и ошибки не содержат названий бирж. Ошибки разбора ответов и
проверка специфичных кодов API остаются в адаптерах соответствующих бирж.

## Конфигурация Kafka

`kafka.brokers` — строка адресов через запятую. Для локального Compose:

```yaml
kafka:
  brokers: "localhost:29092,localhost:39092"
  price_checked_topic: Price-Checked-Topic
```

Для приложения внутри сети Docker вместо localhost используйте
`broker-1:19092,broker-2:19092`. `KafkaConfig.BrokerList()` преобразует строку
в список адресов и убирает пробелы по краям. Пустые элементы запрещены валидацией.

ENV-переопределения: `EXCHANGE_DATA_KAFKA_BROKERS` и
`EXCHANGE_DATA_KAFKA_OUTPUT_TOPIC`. Последнее имя сохранено для совместимости,
но теперь заполняет поле `PriceCheckedTopic`. Приоритет: процесс > .env > YAML > defaults.

## Проверка полного сценария

Сначала поднимите Kafka и Kafka UI из `backend/`:

```bash
docker compose -f deploy/compose.kafka.yaml up -d
```

После готовности брокеров создайте отдельный топик для учебных ID заглушки:

```bash
docker compose -f deploy/compose.kafka.yaml exec broker-1 /opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 --create --if-not-exists --topic exchange-data-smoke --partitions 1 --replication-factor 2
cd services
EXCHANGE_DATA_KAFKA_OUTPUT_TOPIC=exchange-data-smoke go run ./exchange-data/cmd/exchange-data
```

В другом терминале, из `backend/`:

```bash
curl http://localhost:8184/health
docker compose -f deploy/compose.kafka.yaml exec -T broker-1 /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server broker-1:19092 --topic exchange-data-smoke --from-beginning --max-messages 1 --timeout-ms 30000
```

Откройте <http://localhost:8081>, кластер `MarketMind`, топик `exchange-data-smoke`,
вкладку Messages. Событие содержит `event_id` и `items` с ценами, market_id,
биржами и временем получения. При временной недоступности одной биржи публикуются
результаты остальных; если все запросы неуспешны, пустого события не будет.

Остановите Go-приложение через Ctrl+C: должны появиться сообщения об остановке
планировщика и HTTP-сервера, затем процесс завершится. Тестовый топик после
проверки при необходимости можно удалить отдельно. Рабочую БД эти команды не меняют.
