import os
import asyncio
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from users_service.main import create_app
from tests.db_test_utils import create_tables_for_tests


@pytest.fixture
def test_client(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> TestClient:
    test_db_url = os.getenv("USERS_TEST_DB_URL")
    if test_db_url is None:
        db_file = tmp_path / "test_users.db"
        monkeypatch.setenv("USERS_DB_URL", f"sqlite+aiosqlite:///{db_file.as_posix()}")
    else:
        monkeypatch.setenv("USERS_DB_URL", test_db_url)
    # 32+ bytes to avoid PyJWT InsecureKeyLengthWarning in tests
    monkeypatch.setenv("USERS_JWT_SECRET", "test-secret-32-bytes-minimum-123456")
    app = create_app()

    # В тестах создаём таблицы автоматически, без Alembic.
    # В production это делается миграциями.
    asyncio.run(create_tables_for_tests(app.state.container.engine()))

    with TestClient(app) as client:
        yield client
