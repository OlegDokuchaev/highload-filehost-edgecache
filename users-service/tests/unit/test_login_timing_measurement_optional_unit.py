from __future__ import annotations

import os
import statistics
import time

import pytest

from users_service.config import Settings
from users_service.db.session import create_engine, create_session_factory
from users_service.domain.errors import InvalidCredentialsError
from users_service.services.auth import AuthService
from users_service.services.security import SecurityService
from tests.db_test_utils import create_tables_for_tests


pytestmark = pytest.mark.unit

# Константы для timing-теста (вынесены, чтобы не было "магических чисел").
# Мы меряем НЕ абсолютное время, а сравниваем две медианы, поэтому порог задаётся в мс.
TIMING_SAMPLES: int = 40
TIMING_MAX_MEDIAN_DIFF_MS: float = 50.0


def _median_ms(samples_s: list[float]) -> float:
    return statistics.median(samples_s) * 1000.0


@pytest.mark.asyncio
async def test_timing_user_missing_vs_wrong_password_medians_close() -> None:
    """
    Опциональный тест на timing: сравниваем медианы времени.

    По умолчанию пропускается (чтобы не флейкать на разных машинах/нагрузке).
    Включение: USERS_RUN_TIMING_TESTS=1
    """
    if os.getenv("USERS_RUN_TIMING_TESTS") != "1":
        pytest.skip("set USERS_RUN_TIMING_TESTS=1 to run timing measurement tests")

    engine = create_engine("sqlite+aiosqlite:///:memory:")
    await create_tables_for_tests(engine)
    session_factory = create_session_factory(engine)
    security = SecurityService(Settings(jwt_secret="unit-secret-32-bytes-minimum-123456"))
    service = AuthService(session_factory=session_factory, security_service=security)

    # Создаём пользователя
    await service.register(login="User1", password="VeryStrongPass!1")

    # Снимаем несколько замеров; используем perf_counter.
    missing_times: list[float] = []
    wrong_times: list[float] = []

    for _ in range(TIMING_SAMPLES):
        t0 = time.perf_counter()
        with pytest.raises(InvalidCredentialsError):
            await service.login(login="no-such-user", password="WrongPass!1")
        missing_times.append(time.perf_counter() - t0)

        t1 = time.perf_counter()
        with pytest.raises(InvalidCredentialsError):
            await service.login(login="User1", password="WrongPass!1")
        wrong_times.append(time.perf_counter() - t1)

    missing_med = _median_ms(missing_times)
    wrong_med = _median_ms(wrong_times)

    # Порог deliberately широкий: цель — исключить "сильно быстрее" (enumeration),
    # а не гарантировать идеальное равенство на любой машине.
    assert abs(missing_med - wrong_med) < TIMING_MAX_MEDIAN_DIFF_MS

    await engine.dispose()

