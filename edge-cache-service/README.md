# Edge Cache Service

Кэширующий прокси-сервис для раздачи файлов. При первом запросе файл скачивается с origin-сервера, сохраняется на диск и отдаётся клиенту (MISS). Последующие запросы обслуживаются из локального кэша (HIT). При истечении TTL сервис выполняет conditional GET с `If-None-Match` — если origin отвечает `304`, файл отдаётся из кэша без повторной загрузки (REVALIDATED). Request coalescing через файловые блокировки гарантирует, что параллельные запросы одного файла приводят к единственному запросу в origin — в том числе между разными инстансами на одном сервере.

## API

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
│   └── origin/      # HTTP-клиент к origin
└── application/     # Use cases
    └── download/    # DownloadUseCase
```

## Конфигурация

Все параметры задаются через переменные окружения:

| Переменная | Описание | Пример |
|---|---|---|
| `CACHE__DIR` | Директория для кэша | `/tmp/cache` |
| `CACHE__TTL` | Время жизни записи в кэше | `1h`, `30m`, `86400s` |
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
