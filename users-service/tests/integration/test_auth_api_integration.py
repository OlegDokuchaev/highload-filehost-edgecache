import asyncio
import os
from datetime import UTC, datetime, timedelta
from pathlib import Path
from uuid import UUID

import httpx
import pytest
from fastapi.testclient import TestClient
import jwt
from passlib.context import CryptContext
from sqlalchemy import select, update

from users_service.config import Settings
from users_service.db.models import User
from users_service.main import create_app
from users_service.repositories.users import UserRepository
from tests.db_test_utils import create_tables_for_tests


pytestmark = pytest.mark.integration


def test_register_login_verify_flow(test_client: TestClient) -> None:
    register_response = test_client.post(
        "/auth/register",
        json={"login": "MyLogin", "password": "VeryStrongPass!1"},
    )
    assert register_response.status_code == 201
    body = register_response.json()
    assert body["normalized_login"] == "mylogin"
    assert body["user_id"]
    UUID(body["user_id"])

    login_response = test_client.post(
        "/auth/login",
        json={"login": "MyLogin", "password": "VeryStrongPass!1"},
    )
    assert login_response.status_code == 200
    login_body = login_response.json()
    assert login_body["access_token"]
    assert login_body["normalized_login"] == "mylogin"

    verify_response = test_client.post(
        "/auth/verify",
        json={"token": login_body["access_token"]},
    )
    assert verify_response.status_code == 200
    verify_body = verify_response.json()
    assert verify_body["active"] is True
    assert verify_body["normalized_login"] == "mylogin"


def test_register_conflict(test_client: TestClient) -> None:
    payload = {"login": "SameLogin", "password": "VeryStrongPass!1"}
    first = test_client.post("/auth/register", json=payload)
    second = test_client.post("/auth/register", json=payload)

    assert first.status_code == 201
    assert second.status_code == 409


def test_login_invalid_credentials(test_client: TestClient) -> None:
    test_client.post(
        "/auth/register",
        json={"login": "AnotherLogin", "password": "VeryStrongPass!1"},
    )
    response = test_client.post(
        "/auth/login",
        json={"login": "AnotherLogin", "password": "wrong-password"},
    )
    assert response.status_code == 401


@pytest.mark.parametrize(
    "password",
    [
        "NoSpecial123",  # no special char
        "short1A!",  # too short
        "A1!" + ("b" * 126),  # too long (129 chars)
    ],
)
def test_register_weak_password_returns_400(test_client: TestClient, password: str) -> None:
    response = test_client.post(
        "/auth/register",
        json={"login": "WeakPasswordUser", "password": password},
    )
    assert response.status_code == 400
    body = response.json()
    assert isinstance(body.get("detail"), str)
    assert body["detail"]


def test_verify_invalid_token_returns_active_false(test_client: TestClient) -> None:
    response = test_client.post("/auth/verify", json={"token": "not-a-jwt"})
    assert response.status_code == 200
    assert response.json() == {"active": False, "user_id": None, "normalized_login": None}


@pytest.mark.asyncio
async def test_concurrent_register_same_login(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    db_file = tmp_path / "concurrent_users.db"
    monkeypatch.setenv("USERS_DB_URL", f"sqlite+aiosqlite:///{db_file.as_posix()}")
    # 32+ bytes to avoid PyJWT InsecureKeyLengthWarning in tests
    monkeypatch.setenv("USERS_JWT_SECRET", "concurrent-secret-32-bytes-minimum-123")

    app = create_app()
    await create_tables_for_tests(app.state.container.engine())

    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        payload = {"login": "RaceLogin", "password": "VeryStrongPass!1"}

        async def register_once() -> httpx.Response:
            return await client.post("/auth/register", json=payload)

        responses = await asyncio.gather(*[register_once() for _ in range(8)])

    await app.state.container.engine().dispose()

    status_codes = [response.status_code for response in responses]
    assert status_codes.count(201) == 1
    assert status_codes.count(409) == 7


def test_verify_expired_token_returns_active_false(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    db_file = tmp_path / "expired_users.db"
    monkeypatch.setenv("USERS_DB_URL", f"sqlite+aiosqlite:///{db_file.as_posix()}")
    # 32+ bytes to avoid PyJWT InsecureKeyLengthWarning in tests
    monkeypatch.setenv("USERS_JWT_SECRET", "expired-secret-32-bytes-minimum-12345")

    app = create_app()
    asyncio.run(create_tables_for_tests(app.state.container.engine()))
    with TestClient(app) as client:
        register_response = client.post(
            "/auth/register",
            json={"login": "ExpiredUser", "password": "VeryStrongPass!1"},
        )
        assert register_response.status_code == 201

        login_response = client.post(
            "/auth/login",
            json={"login": "ExpiredUser", "password": "VeryStrongPass!1"},
        )
        assert login_response.status_code == 200

        user_id = login_response.json()["user_id"]
        settings = Settings()
        expired_payload = {
            "sub": user_id,
            "normalized_login": "expireduser",
            "iat": int((datetime.now(UTC) - timedelta(minutes=10)).timestamp()),
            "exp": int((datetime.now(UTC) - timedelta(minutes=5)).timestamp()),
        }
        expired_token = jwt.encode(
            expired_payload,
            settings.jwt_secret,
            algorithm=settings.jwt_algorithm,
        )
        verify_response = client.post("/auth/verify", json={"token": expired_token})
        assert verify_response.status_code == 200
        assert verify_response.json()["active"] is False


def test_login_rehashes_legacy_password_hash(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    db_file = tmp_path / "rehash_users.db"
    monkeypatch.setenv("USERS_DB_URL", f"sqlite+aiosqlite:///{db_file.as_posix()}")
    monkeypatch.setenv("USERS_JWT_SECRET", "rehash-secret-32-bytes-minimum-12345")

    app = create_app()
    asyncio.run(create_tables_for_tests(app.state.container.engine()))
    with TestClient(app) as client:
        register_response = client.post(
            "/auth/register",
            json={"login": "RehashUser", "password": "VeryStrongPass!1"},
        )
        assert register_response.status_code == 201
        user_id = UUID(register_response.json()["user_id"])

        legacy_ctx = CryptContext(schemes=["pbkdf2_sha256"], deprecated="auto")
        legacy_hash = legacy_ctx.hash("VeryStrongPass!1")

        async def _set_legacy_hash() -> None:
            session_factory = app.state.container.session_factory()
            async with session_factory() as session:
                await session.execute(
                    update(User).where(User.id == user_id).values(password_hash=legacy_hash)
                )
                await session.commit()

        asyncio.run(_set_legacy_hash())

        login_response = client.post(
            "/auth/login",
            json={"login": "RehashUser", "password": "VeryStrongPass!1"},
        )
        assert login_response.status_code == 200

        async def _read_password_hash() -> str:
            session_factory = app.state.container.session_factory()
            async with session_factory() as session:
                result = await session.execute(
                    select(User.password_hash).where(User.id == user_id)
                )
                return result.scalar_one()

        current_hash = asyncio.run(_read_password_hash())
        assert current_hash.startswith("$argon2")


def test_update_password_hash_conditional_avoids_stale_overwrite(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    """Repository must use WHERE password_hash = expected (CAS), not blind UPDATE."""
    db_file = tmp_path / "cas_users.db"
    monkeypatch.setenv("USERS_DB_URL", f"sqlite+aiosqlite:///{db_file.as_posix()}")
    monkeypatch.setenv("USERS_JWT_SECRET", "cas-secret-32-bytes-minimum-1234567")

    app = create_app()
    asyncio.run(create_tables_for_tests(app.state.container.engine()))

    async def _run() -> None:
        session_factory = app.state.container.session_factory()
        async with session_factory() as session:
            repo = UserRepository(session)
            user = await repo.create_user(
                login="CasUser",
                normalized_login="casuser",
                password_hash="hash_v1",
            )
            await session.commit()
            user_id = user.id

        async with session_factory() as session:
            repo = UserRepository(session)
            n = await repo.update_password_hash(
                user_id=user_id,
                expected_password_hash="wrong_expected",
                new_password_hash="hash_v2",
            )
            assert n == 0
            await session.commit()

        async with session_factory() as session:
            result = await session.execute(
                select(User.password_hash).where(User.id == user_id)
            )
            assert result.scalar_one() == "hash_v1"

        async with session_factory() as session:
            repo = UserRepository(session)
            n = await repo.update_password_hash(
                user_id=user_id,
                expected_password_hash="hash_v1",
                new_password_hash="hash_v2",
            )
            assert n == 1
            await session.commit()

        async with session_factory() as session:
            result = await session.execute(
                select(User.password_hash).where(User.id == user_id)
            )
            assert result.scalar_one() == "hash_v2"

    async def _dispose() -> None:
        await app.state.container.engine().dispose()

    asyncio.run(_run())
    asyncio.run(_dispose())


@pytest.mark.postgres
def test_postgres_uuid_roundtrip(test_client: TestClient) -> None:
    db_url = os.getenv("USERS_TEST_DB_URL", "")
    if not db_url.startswith("postgresql+asyncpg://"):
        pytest.skip("postgres test runs only when USERS_TEST_DB_URL is set to postgres")

    response = test_client.post(
        "/auth/register",
        json={"login": "PostgresUuidUser", "password": "VeryStrongPass!1"},
    )
    assert response.status_code == 201
    body = response.json()
    UUID(body["user_id"])
