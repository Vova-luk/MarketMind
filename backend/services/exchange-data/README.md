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
