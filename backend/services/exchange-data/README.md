# Exchange-Data-Service

Каркас запускает HTTP-сервер состояния на порту `8184`, отвечает на
`GET /health`, пишет `server is alive` через slog каждые 10 секунд и корректно
останавливается по Ctrl+C или SIGTERM.

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

## Следующий этап

gRPC API котировок, опрос бирж и Kafka producer пока не подключены. Секция
`grpc` описывает будущий сервер на порту `9090`; сейчас этот порт не открывается.
`scheduler.interval` относится к будущему опросу бирж, а heartbeat HTTP-сервера
всегда составляет 10 секунд. `/health` проверяет только HTTP-сервер.

БД и Kafka consumer этому сервису по текущей схеме не требуются. Каркас
запускается без внешних подключений и ключей бирж.

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
Сервисный слой и автоматический запуск опроса будут подключены отдельно.

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
Воркер, gRPC и WebSocket ещё не запускаются этим клиентом автоматически.

[Документация OKX: GET / Tickers](https://www.okx.com/docs-v5/en/#order-book-trading-market-data-get-tickers).
