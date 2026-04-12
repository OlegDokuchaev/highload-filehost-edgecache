# Highload Filehost + Rust EdgeCache

Высоконагруженный файлохостинг с edge-кэшированием на Rust и измеримой верификацией производительности (RPS, p95/p99 latency, hit ratio) при горизонтальном масштабировании через consistent hashing.

## Документация
- **Wiki:** `https://github.com/OlegDokuchaev/highload-filehost-edgecache/wiki`
- **API документация:** Swagger у каждого сервиса (`/docs`) и OpenAPI (`/openapi.json`)

## Компоненты
- **Users service:** сервис для работы с пользователями
- **Origin storage service:** сервис для хранения оригинальных файлов
- **EdgeCache service (Rust, Axum/Tokio):** промежуточный сервис-кэш, который обслуживает публичные запросы на скачивание файлов через Интернет
- **Edge Gateway (Rust, apigate):** API-gateway и балансировщик нагрузки, который распределяет входящие запросы между экземплярами EdgeCache-сервисов с помощью consistent hashing по `file_id` (Jump Consistent Hash)

## Быстрый старт

1. Создайте `.env` файлы для каждого сервиса на основе `.env.example`:
```bash
cp users-service/.env.example users-service/.env
cp edge-cache-service/.env.example edge-cache-service/.env
cp edge-gateway/.env.example edge-gateway/.env
cp loadtest/.env.example loadtest/.env
```
Заполните `.env` файлы нужными значениями.

2. Запуск:
```bash
docker compose up -d --build
````

3. Проверка:

* Users: `http://localhost:<users_port>/docs`
* Origin: `http://localhost:<origin_port>/docs`
* EdgeCache: `http://localhost:<edge_port>/docs`
