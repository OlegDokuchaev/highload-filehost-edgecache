# Edge Gateway

API-gateway и балансировщик нагрузки для edge-cache инстансов. Построен на библиотеке [apigate](https://crates.io/crates/apigate). Распределяет запросы на скачивание файлов между edge-cache бэкендами с помощью consistent hashing по `file_id` — один и тот же файл всегда попадает на один и тот же инстанс, что максимизирует cache hit ratio и исключает дублирование данных в кэшах.

## Балансировка

| Компонент | Реализация | Назначение |
|---|---|---|
| Routing | `PathSticky("file_id")` | Извлекает `file_id` из пути запроса как ключ аффинности |
| Balancing | `ConsistentHash` | xxHash3 + Jump Consistent Hash — детерминистически маппит ключ на бэкенд |

Свойства Jump Consistent Hash:
- **Детерминизм** — одинаковый `file_id` всегда попадает на один и тот же бэкенд
- **Минимальное перераспределение** — при добавлении/удалении ноды перемещается только `1/n` ключей
- **Lock-free** — без мьютексов, только атомарные операции
- **O(ln n)** по числу бэкендов

## API

```
GET /download/{file_id}
```

Проксирует запрос на соответствующий edge-cache инстанс. Ответ и заголовки (`X-Cache`, `Content-Type`, `ETag`) транслируются без изменений.

## Архитектура

```
src/
├── main.rs       # Точка входа: загрузка конфига, сборка App, запуск сервера
├── settings.rs   # GatewaySettings (config crate, env vars)
└── routes.rs     # Определение маршрутов (apigate макросы)
```

## Конфигурация

Все параметры задаются через переменные окружения с префиксом `GATEWAY__`:

| Переменная | Описание | Пример |
|---|---|---|
| `GATEWAY__LISTEN_ADDR` | Адрес для прослушивания | `0.0.0.0:3000` |
| `GATEWAY__BACKENDS` | Edge-cache бэкенды (через запятую) | `http://edge-cache-1:3000,http://edge-cache-2:3000` |
| `GATEWAY__REQUEST_TIMEOUT` | Таймаут проксирования | `60s`, `2m` |

## Запуск

### Локально

Требования: Rust 1.88+

```bash
cp .env.example .env
cargo run --release
```

### Docker

```bash
docker build -t edge-gateway .
docker run --env-file .env -p 3000:3000 edge-gateway
```

### Docker Compose

Gateway запускается вместе с edge-cache инстансами из корневого `docker-compose.yml`:

```bash
docker compose up -d --build
```

```
Client :3000 -> edge-gateway -> edge-cache-1 :3000
                              -> edge-cache-2 :3000
                              -> edge-cache-3 :3000
```
