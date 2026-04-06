# Edge Cache Service

Кэширующий прокси-сервис для раздачи файлов. При первом запросе файл скачивается с origin-сервера, сохраняется на диск и отдаётся клиенту (MISS). Последующие запросы обслуживаются из локального кэша (HIT). При истечении TTL сервис выполняет conditional GET с `If-None-Match` — если origin отвечает `304`, файл отдаётся из кэша без повторной загрузки (REVALIDATED). Request coalescing через файловые блокировки (`flock`) гарантирует, что параллельные запросы одного файла приводят к единственному запросу в origin. Работает между инстансами на одном сервере при общем `CACHE__DIR`.

## API

### Документация

| Endpoint | Описание |
|---|---|
| `GET /docs` | Swagger UI — интерактивный просмотр и тестирование API |
| `GET /openapi.json` | OpenAPI-спецификация в формате JSON |

### Endpoints

```
GET /download/{file_id}
```

Заголовки ответа:
- `X-Cache: HIT` — файл из кэша, TTL не истёк
- `X-Cache: MISS` — файл загружен с origin
- `X-Cache: REVALIDATED` — TTL истёк, origin подтвердил актуальность (`304`) или вернул новый контент (`200`)
- `Content-Type` — проксируется с origin-сервера
- `ETag` — проксируется с origin-сервера (если присутствует)

## Архитектура

Гексагональная архитектура (ports & adapters):

```
src/
├── ports/           # Интерфейсы (traits)
│   ├── cache/       # CacheRepo, CacheWriter
│   ├── origin/      # OriginClient
│   └── common.rs    # ByteStream
├── adapters/        # Реализации
│   ├── api/         # HTTP-хэндлеры (axum)
│   ├── cache/       # Дисковый кэш
│   ├── ext/         # Extension traits (LogOnErr)
│   ├── logging/     # Инициализация tracing
│   └── origin/      # HTTP-клиент к origin
└── application/     # Use cases
    └── download/    # DownloadUseCase
```

## Конфигурация

Все параметры задаются через переменные окружения:

| Переменная | Описание | Пример |
|---|---|---|
| `LOG__LEVEL` | Уровень логирования ([`EnvFilter`](https://docs.rs/tracing-subscriber/latest/tracing_subscriber/filter/struct.EnvFilter.html) директива) | `info`, `debug`, `edge_cache_service=trace` |
| `CACHE__DIR` | Директория для кэша | `/tmp/cache` |
| `CACHE__DEFAULT_TTL` | Default TTL (fallback, если origin не отдаёт `Cache-Control`) | `1h`, `30m`, `86400s` |
| `ORIGIN__BASE_URL` | URL origin-сервера | `http://origin:8080` |
| `ORIGIN__TIMEOUT` | Таймаут запроса к origin | `30s` |
| `API__LISTEN_ADDR` | Адрес для прослушивания | `0.0.0.0:3000` |

## Запуск

### Локально

Требования: Rust 1.88+

```bash
# Скопировать и заполнить конфигурацию
cp .env.example .env

# Собрать и запустить
cargo run --release
```

### Docker

```bash
# Собрать образ
docker build -t edge-cache-service .

# Запустить
docker run --env-file .env -p 3000:3000 edge-cache-service
```

## Тесты и проверки

```bash
# Все тесты (unit + integration)
make test

# Unit-тесты
make unit

# Интеграционные тесты
make integration

# Проверка форматирования
make fmt-check

# Линтер
make clippy
```

## Нагрузочное тестирование

Требования: [k6](https://grafana.com/docs/k6/), Node.js

```bash
# Запустить mock origin (генерирует файлы по file_id на лету)
make k6-origin &

# Запустить EdgeCache
cargo run --release &

# Сценарии
make k6-cold       # Cold cache — уникальные файлы, 10 VU × 500 итераций
make k6-warm       # Warm cache — 200 RPS из кэша
make k6-hot        # Hot key — 500 RPS на один файл (coalescing)
```

Результаты сохраняются в `k6/results/`. Отчёт: `k6/PERF-REPORT.md`.
