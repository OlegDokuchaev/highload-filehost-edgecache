# GitHub Issues Stats

Утилита командной строки для статистики по GitHub issues по неделям или месяцам. Считает среднюю длительность в днях для состояний open, closed и «ожидание PR» (`in_pr`): дни накапливаются по фактическому пересечению интервалов жизни issue с границами периода. Результат выводится таблицей в stdout; при указании путей — строятся горизонтальные группированные столбчатые диаграммы (PNG). Для приватных репозиториев и повышения лимитов API нужен токен (`GITHUB_TOKEN` или `--token`). Момент перехода в стадию ожидания PR задаётся `--pr-wait-method`: GraphQL timeline, разбор связанных PR по REST (closing keywords в title/body) или отключение.

## CLI

### Справка


| Команда                 | Описание                             |
| ----------------------- | ------------------------------------ |
| `python main.py --help` | Список аргументов и краткое описание |


### Аргументы


| Аргумент           | Описание                                                                                        |
| ------------------ | ----------------------------------------------------------------------------------------------- |
| `--owner`          | Владелец репозитория (обязательно)                                                              |
| `--repo`           | Имя репозитория (обязательно)                                                                   |
| `--period`         | Агрегация: `week` или `month` (по умолчанию `week`)                                             |
| `--state`          | Фильтр для запроса к API: `all`, `open`, `closed` (по умолчанию `all`)                          |
| `--since`          | Учитывать только issues, обновлённые не раньше даты (ISO 8601, например `2026-01-01T00:00:00Z`) |
| `--token`          | GitHub token; если не задан, берётся `GITHUB_TOKEN` из окружения                                |
| `--pr-wait-method` | Как определять вход в `in_pr`: `auto`, `graphql`, `rest`, `none` (по умолчанию `auto`)          |
| `--counts-png`     | Путь к PNG: количества open / closed / in_pr по периодам                                        |
| `--avg-png`        | Путь к PNG: средние `avg_*_days` по периодам                                                    |


Колонки таблицы (по строкам периода):

- `avg_open_days`, `avg_closed_days`, `avg_in_pr_days` — средние длительности в днях (или пусто, если нет данных)
- `open_issues_count`, `closed_issues_count`, `in_pr_issues_count` — число задач с ненулевым вкладом в метрику за период

## Архитектура

Монолитный скрипт `main.py` (стандартная библиотека + опционально `matplotlib`):

```
main.py
├── fetch_issues / fetch_pull_requests — пагинация GitHub REST API
├── github_request_json — HTTP + заголовки (Bearer при наличии token)
├── build_rest_in_pr_index — индекс «issue → момент in_pr» по текстам связанных PR
├── graphql_issue_in_pr_at — уточнение in_pr через GraphQL timeline (при методе graphql/auto)
├── split_interval_by_period — разбиение интервалов по неделям/месяцам
├── calculate_stats — агрегация в PeriodStats → строки таблицы
├── print_table — вывод в stdout
└── write_grouped_png — экспорт диаграмм (требует matplotlib)
```

## Конфигурация

Основной способ аутентификации — переменная окружения (альтернатива — `--token` в командной строке):


| Переменная     | Описание                                    | Пример    |
| -------------- | ------------------------------------------- | --------- |
| `GITHUB_TOKEN` | Fine-grained или classic PAT для GitHub API | `ghp_...` |


Параметры запуска задаются только аргументами CLI (см. таблицу выше).

## Запуск

### Локально

Требования: Python 3.12+ (рекомендуется; совместимо с 3.x при наличии используемых возможностей языка).

```bash
# Только таблица (токен опционален для публичных репозиториев)
python main.py --owner octocat --repo Hello-World --period week --state all

# С графиками (один раз установить matplotlib)
pip install matplotlib
python main.py \
  --owner octocat \
  --repo Hello-World \
  --period month \
  --counts-png ./counts.png \
  --avg-png ./avg-days.png
```

### Docker

```bash
# Собрать образ
docker build -t github-issues-stats -f dockerfile .

# Запуск (таблица)
docker run --rm -e "GITHUB_TOKEN=${GITHUB_TOKEN}" github-issues-stats \
  --owner octocat \
  --repo Hello-World \
  --period week \
  --state all

# Графики на хост в ./out
mkdir -p ./out
docker run --rm \
  -e "GITHUB_TOKEN=${GITHUB_TOKEN}" \
  -v "$(pwd)/out:/out" \
  github-issues-stats \
  --owner octocat \
  --repo Hello-World \
  --period week \
  --counts-png /out/counts.png \
  --avg-png /out/avg-days.png
```

## Тесты

Отдельного набора unit/integration-тестов в репозитории нет. Быстрая (умеренно долгая) ручная проверка:

```bash
python main.py --help
python main.py --owner OlegDokuchaev --repo highload-filehost-edgecache --period week
```

При ошибках API сообщения выводятся в stderr; код возврата `1` при фатальных ошибках выполнения.