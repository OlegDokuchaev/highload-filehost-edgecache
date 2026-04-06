# Нагрузочное тестирование — Origin vs EdgeCache

Сравнительные нагрузочные тесты для origin (Node.js mock) и edge-cache (Rust, Axum/Tokio) с помощью [Grafana k6](https://grafana.com/docs/k6/).

## Требования

- [k6](https://grafana.com/docs/k6/latest/set-up/install-k6/)
- [Node.js](https://nodejs.org/) (для mock origin)

## Структура

```
loadtest/
├── origin/
│   ├── server.js               # Mock origin — генерирует файлы по prefix в file_id
│   └── start.sh
├── scenarios/
│   ├── origin-baseline.js      # Origin: один файл, constant 500 RPS
│   ├── origin-degrade.js       # Origin: один файл, ramping 100→5000 RPS
│   ├── cache-cold.js           # Cache: уникальные file_id, 100% MISS
│   ├── cache-warm.js           # Cache: seed + constant 200 RPS, 100% HIT
│   ├── cache-hot-key.js        # Cache: один файл, constant 500 RPS
│   └── cache-degrade.js        # Cache: один файл, ramping 100→5000 RPS
├── config.js                   # Общие параметры (URL, VU, RPS, пулы)
├── helpers.js                  # downloadOrigin(), downloadCache(), buildReport()
├── Makefile
├── .env.example
└── results/                    # Результаты (gitignored)
```

## Быстрый старт

```bash
cd loadtest
```

### 1. Запуск mock origin

```bash
make mock-origin &
```

Mock origin слушает на `:8080` и генерирует файлы детерминированно по file_id.
Размер файла определяется по префиксу: `1kb`, `10kb`, `100kb`, `1mb`, `5mb`.

### 2. Запуск EdgeCache

Из директории `edge-cache-service/`:

```bash
# Убедиться, что ORIGIN__BASE_URL=http://localhost:8080 в .env
cargo run --release &
```

### 3. Запуск тестов

```bash
make origin            # Origin baseline
make origin-degrade    # Origin: точка деградации

make cache-cold        # Cache: холодный кэш
make cache-warm        # Cache: прогретый кэш
make cache-hot         # Cache: hot key
make cache-degrade     # Cache: точка деградации
```

## Сценарии

### Origin Baseline (`origin-baseline.js`)

Baseline: все VU бьют в один файл (`1mb-hot-0`) напрямую на origin. Без кэширования — чистая производительность origin.

| Параметр | Значение |
|----------|----------|
| Executor | `constant-arrival-rate` |
| RPS | 500 |
| VUs | 50 |
| Duration | 30s |
| Файл | `1mb-hot-0` |

### Cache Cold (`cache-cold.js`)

Каждый запрос — уникальный `file_id`. Кэш пуст, 100% MISS. Показывает latency при промахе кэша (edge-cache → origin → запись в кэш).

| Параметр | Значение |
|----------|----------|
| Executor | `per-vu-iterations` |
| VUs | 10 |
| Итераций на VU | 500 |

### Cache Warm (`cache-warm.js`)

Фаза seed заполняет кэш 200 файлами. Затем constant RPS — все запросы из кэша (100% HIT).

| Параметр | Значение |
|----------|----------|
| Seed | 1 VU × 200 файлов |
| Executor (warm) | `constant-arrival-rate` |
| RPS | 200 |
| VUs | 20 |
| Duration | 30s |

### Cache Hot Key (`cache-hot-key.js`)

Все VU бьют в один закэшированный файл. Проверяет request coalescing и производительность кэша на горячем ключе. Прямое сравнение с origin-baseline — те же параметры (500 RPS, 50 VUs, один файл).

| Параметр | Значение |
|----------|----------|
| Executor | `constant-arrival-rate` |
| RPS | 500 |
| VUs | 50 |
| Duration | 30s |
| Файл | `1mb-hot-0` |

### Origin Degrade (`origin-degrade.js`)

Нахождение точки деградации origin. RPS растёт ступенями на одном файле. Тест **автоматически останавливается**, если p99 latency превышает порог или error rate слишком высокий.

| Параметр | Значение по умолчанию | Env-переменная |
|----------|-----------------------|----------------|
| Start RPS | 100 | `DEGRADE_START_RPS` |
| Шаг RPS | +200 | `DEGRADE_STEP_RPS` |
| Кол-во ступеней | 10 | `DEGRADE_STEPS` |
| Длительность ступени | 30s | `DEGRADE_STAGE_SEC` |
| Abort p99 >= | 1000 ms | `DEGRADE_P99_LIMIT` |
| Abort error >= | 1% | `DEGRADE_ERR_LIMIT` |
| Delay before abort | 10s | `DEGRADE_ABORT_DELAY` |
| Файл | `1mb-degrade-0` | `DEGRADE_FILE_ID` |

Ступени (по умолчанию): 100 → 300 → 500 → 700 → 900 → 1100 → 1300 → 1500 → 1700 → 1900 RPS.

### Cache Degrade (`cache-degrade.js`)

Нахождение точки деградации edge-cache. Файл предварительно кэшируется (seed), затем RPS растёт ступенями. Все запросы — HIT. Автоматическая остановка по тем же порогам.

Параметры идентичны Origin Degrade (общие env-переменные `DEGRADE_*`).

## Конфигурация

Все параметры настраиваются через переменные окружения (см. `.env.example`):

```bash
# Пример: запуск cache hot key с 1000 RPS
HOT_RPS=1000 k6 run scenarios/cache-hot-key.js
```

## Метрики

| Метрика | Описание |
|---------|----------|
| `download_latency` | Custom Trend — время скачивания (avg, med, p90, p95, p99) |
| `http_reqs` | Количество запросов и RPS |
| `cache_hits` | Количество ответов с `X-Cache: HIT` |
| `cache_misses` | Количество ответов с `X-Cache: MISS` |
| `cache_revalidated` | Количество ответов с `X-Cache: REVALIDATED` |
| `hit_ratio` | `HIT / (HIT + MISS + REVALIDATED)` |

## Результаты

Каждый сценарий сохраняет:
- `results/<scenario>.md` — человекочитаемый отчёт
- `results/<scenario>.json` — полный JSON с метриками k6

Сводный анализ и сравнение Origin vs Cache — в [RESULTS.md](RESULTS.md).

## Makefile targets

| Target | Описание |
|--------|----------|
| `mock-origin` | Запуск mock origin |
| `origin` | Origin baseline (один файл, 500 RPS) |
| `origin-degrade` | Origin: точка деградации |
| `cache-cold` | Cache: холодный кэш |
| `cache-warm` | Cache: прогретый кэш |
| `cache-hot` | Cache: hot key (один файл, 500 RPS) |
| `cache-degrade` | Cache: точка деградации |
| `clean` | Удалить results/*.json и results/*.md |
