from __future__ import annotations

import pytest

from users_service.config import Settings
from users_service.db.session import create_engine, create_session_factory
from users_service.domain.errors import InvalidCredentialsError
from users_service.services.auth import AuthService
from users_service.services.security import SecurityService
from tests.db_test_utils import create_tables_for_tests


pytestmark = pytest.mark.unit


@pytest.mark.asyncio
async def test_login_user_not_found_still_verifies_password(monkeypatch: pytest.MonkeyPatch) -> None:
    """Проверяем mitigation: при user is None всё равно вызывается verify_password()."""
    engine = create_engine("sqlite+aiosqlite:///:memory:")
    await create_tables_for_tests(engine)
    session_factory = create_session_factory(engine)

    security = SecurityService(Settings(jwt_secret="unit-secret-32-bytes-minimum-123456"))

    calls: list[tuple[str, str]] = []
    original_verify = security.verify_password

    def spy_verify(plain_password: str, password_hash: str) -> bool:
        calls.append((plain_password, password_hash))
        return original_verify(plain_password, password_hash)

    monkeypatch.setattr(security, "verify_password", spy_verify)

    service = AuthService(session_factory=session_factory, security_service=security)

    with pytest.raises(InvalidCredentialsError):
        await service.login(login="missing-user", password="WrongPass!1")

    assert len(calls) == 1
    assert calls[0][0] == "WrongPass!1"
    assert calls[0][1] == security.dummy_password_hash

    await engine.dispose()

