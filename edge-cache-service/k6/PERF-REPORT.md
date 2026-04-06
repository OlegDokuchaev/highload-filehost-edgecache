# Отчёт о производительности — EdgeCache Service

## Окружение

| Параметр            | Значение                                    |
|---------------------|---------------------------------------------|
| EdgeCache           | edge-cache-service (Rust, axum, tokio)      |
| Origin              | Node.js mock (динамическая генерация по file_id) |
| Машина              | Apple M4 Max                                |
| ОС                  | macOS (Darwin 24.6.0)                       |
| CACHE__DEFAULT_TTL  | 1h                                          |
| Размеры файлов      | 1 KB, 10 KB, 100 KB, 500 KB, 1 MB, 5 MB    |
| Транспорт           | localhost (loopback)                        |

## Инструмент

[Grafana k6](https://grafana.com/docs/k6/) — нагрузочное тестирование HTTP-сервисов.

Метрики собираются на стороне k6 по заголовку `X-Cache` из ответов EdgeCache:
- **Hit ratio** = `HIT / (HIT + MISS + REVALIDATED)`
- **Origin offload** = `HIT / (HIT + MISS + REVALIDATED)` (только HIT не обращается в origin)
- **RPS** = `http_reqs.rate` (встроенная метрика k6)
- **p95/p99** = перцентили custom Trend `download_latency`

## Сценарии и параметры

### 1. Cold Cache

Каждый запрос — уникальный `file_id` (формат `{size}-v{VU}i{ITER}`). Кэш пуст, 100% MISS.
Цель: baseline latency при обращении в origin + запись на диск.

| Параметр         | Значение                  |
|------------------|---------------------------|
| Executor         | `per-vu-iterations`       |
| VUs              | 10                        |
| Итераций на VU   | 500                       |
| Всего запросов   | 5 000                     |
| Размеры файлов   | взвешенный микс (1 KB — 5 MB) |

**Результаты:**

| Метрика          | Значение     |
|------------------|--------------|
| Всего запросов   | 5 000        |
| RPS              | 1 204.6      |
| p95 задержка     | 17.3 ms      |
| p99 задержка     | 26.5 ms      |
| Cache HIT        | 0            |
| Cache MISS       | 5 000        |
| REVALIDATED      | 0            |
| Hit ratio        | 0.0%         |
| Origin offload   | 0.0%         |

100% MISS — подтверждает, что каждый file_id уникален и кэш действительно холодный.

### 2. Warm Cache

Фиксированный пул из 200 файлов. Фаза seed (1 VU) заполняет кэш, затем гарантированные 200 RPS из кэша.
Цель: latency при отдаче из дискового кэша, hit ratio.

| Параметр         | Значение                  |
|------------------|---------------------------|
| Seed             | 1 VU × 200 файлов        |
| Executor (warm)  | `constant-arrival-rate`   |
| Target RPS       | 200                       |
| VUs              | 20 (pre-allocated)        |
| Длительность     | 30 с                      |
| Start time       | 10 с (после seed)         |

**Результаты:**

| Метрика          | Значение     |
|------------------|--------------|
| Всего запросов   | 6 000        |
| RPS              | 155.0        |
| p95 задержка     | 16.7 ms      |
| p99 задержка     | 18.7 ms      |
| Cache HIT        | 6 000        |
| Cache MISS       | 0            |
| REVALIDATED      | 0            |
| Hit ratio        | 100.0%       |
| Origin offload   | 100.0%       |

100% HIT — все запросы обслужены из дискового кэша без обращения в origin.

### 3. Hot Key

Все VU бьют в один файл (`1mb-hot-0`, 1 MB) с гарантированным RPS.
Цель: проверка request coalescing — только 1 запрос должен уйти в origin.

| Параметр         | Значение                  |
|------------------|---------------------------|
| Executor         | `constant-arrival-rate`   |
| Target RPS       | 500                       |
| VUs              | 50 (pre-allocated)        |
| Длительность     | 30 с                      |
| Горячий файл     | `1mb-hot-0` (1 MB)        |

**Результаты:**

| Метрика          | Значение     |
|------------------|--------------|
| Всего запросов   | 15 001       |
| RPS              | 499.9        |
| p95 задержка     | 4.8 ms       |
| p99 задержка     | 7.6 ms       |
| Cache HIT        | 15 000       |
| Cache MISS       | 1            |
| REVALIDATED      | 0            |
| Hit ratio        | 100.0%       |
| Origin offload   | 100.0%       |
| Coalescing       | **effective** |

1 MISS на 15 001 запросов — request coalescing через `flock` работает корректно.

## Сравнение Cold vs Warm

| Метрика      | Cold Cache | Warm Cache | Улучшение      |
|--------------|------------|------------|----------------|
| p95 задержка | 17.3 ms    | 16.7 ms    | -3.5%          |
| p99 задержка | 26.5 ms    | 18.7 ms    | **-29.4%**     |
| RPS          | 1 204.6    | 155.0 *    | —              |
| Hit ratio    | 0.0%       | 100.0%     | **+100 п.п.**  |

\* Warm RPS ограничен target rate (200 RPS), фактический throughput выше.

Основное улучшение — в хвостовых задержках (p99: -29.4%) и полном исключении обращений к origin.

## Выводы

1. **Warm cache** показывает улучшение p99 на 29.4% относительно cold cache и 100% hit ratio.
2. **Hot key** подтверждает работу request coalescing: 1 MISS на 15 001 запросов (файловая блокировка через `flock`).
3. **Origin offload** при warm/hot cache: 100% — все запросы обслуживаются локально.
4. Кэширование эффективно для файлов всех размеров (1 KB — 5 MB) в рамках тестового набора.

## Как запустить

```bash
# 1. Запустить mock origin (Node.js)
make k6-origin &

# 2. Запустить EdgeCache
cargo run --release &

# 3. Запуск сценариев
make k6-cold       # Cold cache
make k6-warm       # Warm cache
make k6-hot        # Hot key
```

Результаты сохраняются в `k6/results/`.
