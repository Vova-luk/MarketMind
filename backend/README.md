# Backend MarketMind

## Быстрый запуск в Docker

Нужны Docker с запущенным daemon и Docker Compose 2.20+.

```bash
cp .env.example .env
docker compose --env-file .env -f deploy/compose.yaml up --build -d
curl -i http://localhost:8080/health
```

Compose запускает PostgreSQL и API. Конфиг YAML монтируется в контейнер только для чтения. Логи по умолчанию пишутся в stdout

```bash
docker compose --env-file .env -f deploy/compose.yaml logs -f api
docker compose --env-file .env -f deploy/compose.yaml down
``` 

## PostgreSQL в Docker, API локально

Нужен Go версии из `services/go.mod` (1.27.3).

```bash
cp .env.example .env
docker compose --env-file .env -f deploy/compose.postgres.yaml up -d --wait
cd services
go run ./api/cmd -config ./api/config/config.yaml -env-file ../.env
```