# Bot-Service

Каркас сервиса запускает HTTP-сервер на порту `8183`, отвечает на `GET /health`,
пишет `server is alive` через slog каждые 10 секунд и корректно останавливается
по Ctrl+C или SIGTERM.

Из `backend/services`:

```bash
go run ./bot/cmd/bot
```

Для локальных переопределений:

```bash
cp bot/.env.example bot/.env
go run ./bot/cmd/bot -config bot/config/config.yaml -env-file bot/.env
```

Приоритет настроек: окружение процесса > явно переданный `.env` > YAML > defaults.
YAML загружается в структуры пакета `internal/config`; `.env` читает godotenv.

Проверка HTTP-сервера:

```bash
curl http://localhost:8183/health
```

По архитектуре Bot-Service будет получать события из `Price-Changed-Topic`
через Kafka consumer и отправлять сообщения через Telegram adapter. Подключения
к Kafka и Telegram и обработка уведомлений в этот каркас ещё не включены.
Поэтому для запуска каркаса токен Telegram и работающий Kafka не требуются.
`/health` проверяет только HTTP-сервер, а не доступность этих интеграций.
