# Высоконагруженный файлохостинг с edge-кэшированием на Rust и измеримой верификацией производительности (RPS, p95/p99 latency, hit ratio) при горизонтальном масштабировании через consistent hashing

---

# ТЗ

## 0) Документация API

У каждого сервиса обязательно:

* `GET /openapi.json`
* `GET /docs` — Swagger UI

---

## 1) Users service

Сервис аутентификации: регистрирует пользователя по login/password, выдаёт JWT access token на ограниченное время и предоставляет внутреннюю проверку токена для остальных сервисов.

### POST `/auth/register`

Создать пользователя.

**Request JSON**

```json
{ "login": "MyLogin", "password": "secret" }
```

**Response 201 JSON**

```json
{ "user_id": "uuid", "login": "MyLogin", "normalized_login": "mylogin" }
```

**Errors**

* 400 invalid input
* 409 normalized_login already exists

---

### POST `/auth/login`

Логин и выдача access JWT (без refresh).

**Request JSON**

```json
{ "login": "MyLogin", "password": "secret" }
```

**Response 200 JSON**

```json
{
  "access_token": "jwt",
  "user_id": "uuid",
  "normalized_login": "mylogin"
}
```

**Errors**

* 401 invalid credentials

---

### POST `/auth/verify`

Внутренняя ручка для сервис-к-сервису: проверить токен.

**Request JSON**

```json
{ "token": "jwt" }
```

**Response 200 JSON** (всегда 200)

```json
{ "active": true, "user_id": "uuid", "normalized_login": "mylogin" }
```

или

```json
{ "active": false }
```

---

## 2) Origin storage service

Сервис источника правды (origin): принимает upload файла, хранит оригиналы и отдаёт их по GET /files/{file_id}. Он обязан выставлять HTTP-заголовки кэширования (Cache-Control, ETag) и поддерживать условные запросы (If-None-Match → 304 Not Modified), чтобы кэш мог корректно ревалидировать данные, а не всегда перекачивать файл заново.

### POST `/files`

Загрузка файла и создание записи. Требует JWT.

**Headers**

* `Authorization: Bearer <access_token>`

**Body**

* `multipart/form-data`:

  * `file` (binary, required)

**Auth**

* Origin вызывает Users `POST /auth/verify` и берёт `user_id` из ответа при `active=true`.

**Response 201 JSON**

```json
{
  "file_id": "uuid",
  "owner_user_id": "uuid",
  "filename": "video.mp4",
  "content_type": "video/mp4",
  "size_bytes": 123456,
  "created_at": "2026-02-13T14:10:00Z",
  "download_url": "https://<cdn-host>/download/<file_id>"
}
```

**Errors**

* 401 token invalid
* 400 missing file

---

### GET `/users/me/files`

Список файлов текущего пользователя. Требует JWT.

**Headers**

* `Authorization: Bearer <access_token>`

**Response 200 JSON**

```json
{
  "items": [
    {
      "file_id": "uuid",
      "filename": "video.mp4",
      "content_type": "video/mp4",
      "size_bytes": 123456,
      "created_at": "2026-02-13T14:10:00Z",
      "download_url": "https://<cdn-host>/download/<file_id>"
    }
  ],
  "total": 1
}
```

**Errors**

* 401 token invalid

---

### GET `/files/{file_id}`  (public, origin download endpoint)

Отдаёт оригинал файла. Без авторизации (EdgeCache ходит сюда).

**Response 200**

* body = bytes
* headers (обязательно):

  * `Content-Type: <content_type>`
  * `Cache-Control: public, max-age=<seconds>`
  * `ETag: "<opaque>"`

**Conditional GET**

* Request: `If-None-Match: "<etag>"`
* Response: `304 Not Modified` (без тела), если ресурс не менялся

**Errors**

* 404 not found

---

## 3) EdgeCache service (Rust: Axum/Tokio)

### GET `/download/{file_id}` (public)

Публичное скачивание по постоянной ссылке.

**Поведение**

* **HIT**: отдача из локального дискового кэша
* **MISS**: запрос в Origin `GET /files/{file_id}` и **стриминг чанками** клиенту с одновременной записью в temp → atomic rename
* **REVALIDATED**: если TTL истёк — conditional GET в Origin (`If-None-Match`), при `304` продлеваем TTL и отдаём кэш
* **Request coalescing**: на одном инстансе EdgeCache при параллельных запросах одного `file_id` в Origin идёт **ровно 1** запрос, остальные ждут результат

**Response headers**

* `X-Cache: HIT | MISS | REVALIDATED | BYPASS`
* пробросить: `Content-Type`, желательно `ETag`

---

### GET `/metrics`

* `/metrics` → метрики

---

## 4) NGINX перед EdgeCache

Балансировка нескольких инстансов EdgeCache через **`hash … consistent` (ketama consistent hashing)** по ключу `file_id`. Цель: при изменении пула нод ремаппится мало ключей → меньше cache misses, полезно именно для кэширующих серверов

---

## 5) Нагрузочное тестирование (обязательно)

Инструмент: **Grafana k6**. Метрики: `http_req_duration` (latency), `http_req_failed` (error rate), RPS/throughput

**Сценарии**

1. Cold cache: первая серия скачиваний набора файлов
2. Warm cache: повтор той же серии (hit ratio ↑, p95/p99 ↓)
3. Hot key: много параллельных скачиваний одного `file_id` (проверка coalescing)

**Что сдаём по результатам**

* k6: RPS, p95/p99 `http_req_duration`, `http_req_failed`
* EdgeCache `/metrics`: hit/miss/revalidated, origin request rate, inflight keys
