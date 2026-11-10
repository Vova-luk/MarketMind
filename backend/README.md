# Backend MarketMind

[Контракт API для Telegram Mini App](services/api/README.md)

## Быстрый запуск в Docker

Нужны Docker с запущенным daemon и Docker Compose 2.20+.

```bash
cp .env.example .env
docker compose --env-file .env -f deploy/compose.yaml up --build -d
curl -i http://localhost:8080/health
```

Compose запускает PostgreSQL, Kafka, Kafka UI и API. Конфиг YAML монтируется в контейнер только для чтения. Логи по умолчанию пишутся в stdout

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

## Служебный gRPC-метод API

`GetMarketsToCheck` возвращает рынки активных бирж с хотя бы одной подпиской,
сгруппированные по `exchange_name`. Контракт и Go-код находятся в
[`services/proto/api/api.proto`](services/proto/api/api.proto) и `services/gen/go/api`.

Локальный адрес по умолчанию `127.0.0.1:50051`. В сети Compose метод доступен
по `api:50051`. Для локальной проверки выполните команду из `backend/services`.

```bash
grpcurl -plaintext -import-path proto -proto api/api.proto -d '{}' \
  127.0.0.1:50051 marketmind.api.v1.MarketCatalogService/GetMarketsToCheck
```

После изменения контракта перегенерируйте Go-код из `backend/services`.
Команда использует `protoc`, `protoc-gen-go` и `protoc-gen-go-grpc` из `PATH`.

```bash
protoc -I proto --go_out=. --go_opt=module=marketmind \
  --go-grpc_out=. --go-grpc_opt=module=marketmind proto/api/api.proto
```

## Миграции базы данных

API автоматически применяет миграции после подключения к PostgreSQL и до
запуска HTTP-сервера. SQL общей схемы хранятся в `services/utils/migrations`.
Формат, ручной откат и полный цикл проверки описаны в
[инструкции по миграциям](services/utils/migrations/README.md).

## Kafka и Kafka UI

Команды выполняются из каталога `backend/`.

### Запуск

```bash
docker compose -f deploy/compose.kafka.yaml up -d
docker compose -f deploy/compose.kafka.yaml ps
docker compose -f deploy/compose.kafka.yaml logs -f broker-1 broker-2
```

Kafka UI доступен на <http://localhost:8081>. В кластере `MarketMind` можно
посмотреть топики, сообщения, брокеры и отставание групп консьюмеров.
Контейнеры запускаются по порядку, но `depends_on` не проверяет готовность Kafka:
перед созданием топиков дождитесь запуска обоих брокеров. Если они ещё
запускаются, повторите команду после проверки логов.

Адреса для подключения:

- Go-сервисы, запущенные локально: `localhost:29092,localhost:39092`.
- Сервисы в сети этого Compose: `broker-1:19092,broker-2:19092`.

Общий `deploy/compose.yaml` уже включает Kafka: при полном запуске бэкенда
отдельно запускать её не нужно.

### Создание топиков

```bash
for topic in Price-Checked-Topic Price-Changed-Topic; do
  docker compose -f deploy/compose.kafka.yaml exec -T broker-1 \
    /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server broker-1:19092 \
    --create --if-not-exists --topic "$topic" \
    --partitions 3 --replication-factor 2
done
```

`Price-Checked-Topic` передаёт цены от Exchange-Data-Service в Tracking-Service,
а `Price-Changed-Topic` — события от Tracking-Service в Bot-Service.
Каждый топик создаётся с тремя партициями и двумя репликами. Повторный запуск
не удаляет данные и не меняет настройки существующих топиков.
Создавайте топики до запуска продюсеров, чтобы Kafka не создала их автоматически
с настройками по умолчанию.

Проверить список топиков и реплики:

```bash
docker compose -f deploy/compose.kafka.yaml exec broker-1 /opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 --list
docker compose -f deploy/compose.kafka.yaml exec broker-1 /opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 --describe --topic Price-Checked-Topic
```

### Проверка отправки и чтения сообщений

Для экспериментов используйте отдельный топик, чтобы тестовый JSON не попал
в обработку рабочим сервисам:

```bash
docker compose -f deploy/compose.kafka.yaml exec broker-1 /opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 --create --if-not-exists --topic kafka-test --partitions 1 --replication-factor 2
printf '%s\n' '{"event":"test","price":"65000.50"}' | docker compose -f deploy/compose.kafka.yaml exec -T broker-1 /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server broker-1:19092 --topic kafka-test --producer-property acks=all
docker compose -f deploy/compose.kafka.yaml exec -T broker-1 /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server broker-1:19092 --topic kafka-test --from-beginning --max-messages 1 --timeout-ms 15000
```

Последняя команда должна вывести отправленный JSON. При повторных отправках
она может снова показать первую старую запись, так как читает с начала топика.
Сообщения также видны в Kafka UI: **Topics → kafka-test → Messages**.
Kafka сохраняет сообщения после чтения до истечения срока хранения.
Для проверки обработки сервисом дополнительно смотрите его логи и результат
операции: изменения в БД или доставленное уведомление.

### Остановка и данные

```bash
docker compose -f deploy/compose.kafka.yaml stop
docker compose -f deploy/compose.kafka.yaml start
```

У каждого брокера и контроллера свой именованный volume. Обычный `down`
сохраняет эти данные, а `down -v` удаляет volumes вместе с сообщениями и
метаданными. При двух контроллерах для работы кворума нужны оба.
