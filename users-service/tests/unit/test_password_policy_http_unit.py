"""Проверка маппинга PasswordPolicyError -> HTTP 400 на уровне приложения."""

import json
from pathlib import Path

import pytest
from fastapi import Request
from fastapi.testclient import TestClient

from users_service.domain.errors import PasswordPolicyError
from users_service.main import create_app, password_policy_exception_handler


pytestmark = pytest.mark.unit


@pytest.mark.asyncio
async def test_password_policy_exception_handler_returns_400_json() -> None:
    exc = PasswordPolicyError("password length must be between 12 and 128")
    scope = {
        "type": "http",
        "asgi": {"version": "3.0", "spec_version": "2.3"},
        "method": "GET",
        "path": "/",
        "raw_path": b"/",
        "query_string": b"",
        "headers": [],
    }
    request = Request(scope)
    response = await password_policy_exception_handler(request, exc)

    assert response.status_code == 400
    body = json.loads(response.body.decode())
    assert body == {"detail": str(exc)}


def test_create_app_registers_password_policy_handler() -> None:
    app = create_app()
    assert PasswordPolicyError in app.exception_handlers
    assert app.exception_handlers[PasswordPolicyError] is password_policy_exception_handler


def test_unhandled_password_policy_error_returns_400_via_test_client(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    """Имитация вызова сервиса в обход локального try/except в роуте."""
    db_file = tmp_path / "test_users.db"
    monkeypatch.setenv("USERS_DB_URL", f"sqlite+aiosqlite:///{db_file.as_posix()}")
    # 32+ bytes to avoid PyJWT InsecureKeyLengthWarning in tests
    monkeypatch.setenv("USERS_JWT_SECRET", "test-secret-32-bytes-minimum-123456")

    app = create_app()

    @app.get("/__test_password_policy")
    async def boom() -> None:
        raise PasswordPolicyError("password must contain a digit")

    with TestClient(app) as client:
        response = client.get("/__test_password_policy")

    assert response.status_code == 400
    assert response.json() == {"detail": "password must contain a digit"}
