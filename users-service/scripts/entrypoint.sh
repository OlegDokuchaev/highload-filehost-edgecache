#!/bin/sh
echo "Запуск миграций Alembic..."
alembic upgrade head
echo "Старт users-service..."
exec uvicorn "users_service.main:create_app" --factory --host "0.0.0.0" --port "8081"
