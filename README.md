# Highload Filehost + Rust EdgeCache

Высоконагруженный файлохостинг с edge-кэшированием на Rust и измеримой верификацией производительности (RPS, p95/p99 latency, hit ratio) при горизонтальном масштабировании через consistent hashing.

## Документация
- **Wiki:** `https://github.com/OlegDokuchaev/highload-filehost-edgecache/wiki`
- **API документация:** Swagger у каждого сервиса (`/docs`) и OpenAPI (`/openapi.json`)

## Компоненты
- **Users service:** сервис для работы с пользователями
- **Origin storage service:** сервис для хранения оригинальных файлов
- **EdgeCache service (Rust, Axum/Tokio):** промежуточный сервис-кэш, который обслуживает публичные запросы на скачивание файлов через Интернет
- **NGINX:** балансировщик, который распределяет входящие запросы между экземплярами EdgeCache-сервисов, обеспечивая равномерную загрузку с помощью консистентного хеширования

## Быстрый старт

1. Запуск:
```bash
docker compose up -d --build
````

2. Проверка:

* Users: `http://localhost:<users_port>/docs`
* Origin: `http://localhost:<origin_port>/docs`
* EdgeCache: `http://localhost:<edge_port>/healthz`
