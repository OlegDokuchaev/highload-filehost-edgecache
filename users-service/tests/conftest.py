from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from users_service.main import create_app


@pytest.fixture
def test_client(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> TestClient:
    db_file = tmp_path / "test_users.db"
    monkeypatch.setenv("USERS_DB_URL", f"sqlite+aiosqlite:///{db_file.as_posix()}")
    monkeypatch.setenv("USERS_JWT_SECRET", "test-secret")
    app = create_app()
    with TestClient(app) as client:
        yield client
