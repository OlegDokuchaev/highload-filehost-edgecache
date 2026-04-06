# Users Service

Изолированная рабочая папка для реализации `Users service` в репозитории `highload-filehost-edgecache`.

## Быстрые команды

```bash
cd users-service

cp .env.example .env

docker compose up -d --build

curl http://localhost:8001/healthz

python -m pip install -e .[dev]
pytest
mypy src

powershell -ExecutionPolicy Bypass -File .\scripts\cleanup.ps1
bash ./scripts/cleanup.sh
```

Полезные тестовые команды:

```bash
pytest tests/unit -q
pytest tests/integration -q
```

## 1) Стек и архитектурный минимум

- Язык: `Python 3.12+`
- Web: `FastAPI`
- DB: `PostgreSQL`
- ORM: `SQLAlchemy 2.x (async)` (минимум `2.0.40`)
- Валидация: `Pydantic v2`
- DI: `dependency-injector`
- Тесты: `pytest`
- Типизация: `mypy --strict`

Структура:

```text
src/users_service/
  api/            # роуты + pydantic схемы
  db/             # engine/session/models/init
  domain/         # доменные ошибки
  repositories/   # доступ к данным
  services/       # auth/security логика
  container.py    # DI-контейнер
  config.py       # Settings (env)
  main.py         # FastAPI app factory
tests/
```

## 3) Переменные окружения

`.env.example`:

- `USERS_APP_HOST=0.0.0.0`
- `USERS_APP_PORT=8001`
- `USERS_DB_URL=postgresql+asyncpg://users:users@postgres:5432/users_db`
- `USERS_JWT_SECRET=change-me-please`
- `USERS_JWT_ALGORITHM=HS256`
- `USERS_ACCESS_TOKEN_EXPIRE_MINUTES=60`

Важно: `USERS_JWT_SECRET` из примера небезопасен для production. Перед деплоем
обязательно задай уникальный длинный секрет.

## 4) Политика паролей

Пароль должен:
- иметь длину от 12 до 128 символов;
- содержать минимум 1 заглавную букву;
- содержать минимум 1 строчную букву;
- содержать минимум 1 цифру;
- содержать минимум 1 специальный символ.

При нарушении политики сервис возвращает `400`.

## 4.1) Хеширование паролей (алгоритм)

Используется `argon2id` (через `passlib[argon2]`) как основной алгоритм.

OWASP приоритизирует: `argon2id > bcrypt > scrypt > pbkdf2`. Здесь выбран самый предпочтительный вариант.

### Стратегия миграции паролей

В сервисе включён staged-подход:
- `argon2` используется для новых/обновлённых хешей;
- legacy `pbkdf2_sha256` принимается как deprecated;
- при успешном логине выполняется `verify_and_update`, и хеш прозрачно обновляется до `argon2`.

Ops-checklist:
1. Убедиться, что приложение и воркеры задеплоены с этой dual-scheme конфигурацией.
2. Мониторить долю legacy-хешей в БД (должна стремиться к нулю).
3. После завершения миграции выпустить отдельный релиз, удаляющий legacy-схему.

## 5) Локальный запуск

1. Создать `.env`:

```bash
cp .env.example .env
```

2. Запустить PostgreSQL + сервис:

```bash
docker compose up -d --build
```

3. Проверить:

- `http://localhost:8001/healthz`
- `http://localhost:8001/docs`
- `http://localhost:8001/openapi.json`

## 6) Миграции БД

В production схема БД обновляется через Alembic.

Конфигурация в репозитории:
- `alembic.ini`
- `alembic/env.py` (async SQLAlchemy/asyncpg)
- `alembic/versions/bebd0256e49e_initial.py` (initial migration)

Важно: `USERS_DB_URL=...@postgres:5432/...` работает **внутри docker compose**.
Если ты запускаешь `alembic` на Windows-хосте, используй проброшенный порт:
`localhost:5433` (см. `docker-compose.yml`).

Пример для bash:

```bash
export USERS_DB_URL="postgresql+asyncpg://users:users@localhost:5433/users_db"
alembic upgrade head
```

Пример для PowerShell:

```powershell
$env:USERS_DB_URL="postgresql+asyncpg://users:users@localhost:5433/users_db"
alembic upgrade head
```

В Docker миграции запускаются автоматически перед стартом сервиса (см. `scripts/entrypoint.sh`).
`docker-compose.yml` запускает сервис через этот entrypoint.

`Base.metadata.create_all` не используется в production-коде приложения.
Создание таблиц через `create_all` оставлено только в тестовых утилитах (`tests/db_test_utils.py`).

## 7) Тесты и типизация

```bash
pip install -e .[dev]
pytest
mypy src
```

Отдельный запуск:

```bash
pytest tests/unit -q
pytest tests/integration -q
```

### Интеграционные тесты с PostgreSQL

Часть сценариев помечена `@pytest.mark.postgres` и **пропускается**, если не задан
`USERS_TEST_DB_URL` на реальный PostgreSQL (например `postgresql+asyncpg://...`).
Чтобы проверить нативный UUID и поведение на Postgres:

```bash
set USERS_TEST_DB_URL=postgresql+asyncpg://user:pass@localhost:5432/users_test
pytest -m postgres -q
```

(В bash: `export USERS_TEST_DB_URL=...`.) База должна существовать; таблицы
создаются фикстурами тестов.

## 8) UUID и совместимость БД

Идентификатор пользователя хранится как UUID. В модели используется явный тип
`GUID` с диалектным fallback:
- PostgreSQL: нативный UUID-тип;
- SQLite: `CHAR(36)` с преобразованием в/из UUID.

Это позволяет одинаково работать с UUID в production (PostgreSQL) и локальных
интеграционных тестах (SQLite).

## 9) Очистка лишних файлов

Скрипты очистки артефактов разработки:

- PowerShell: `scripts/cleanup.ps1`
- Bash: `scripts/cleanup.sh`

Запуск (PowerShell):

```powershell
.\scripts\cleanup.ps1
```

Предпросмотр без удаления:

```powershell
.\scripts\cleanup.ps1 -DryRun
```

Запуск (bash):

```bash
bash ./scripts/cleanup.sh
```

Предпросмотр без удаления:

```bash
bash ./scripts/cleanup.sh --dry-run
```
