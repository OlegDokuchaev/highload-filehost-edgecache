from __future__ import annotations

from sqlalchemy.ext.asyncio import AsyncEngine

from users_service.db.base import Base
from users_service.db import models  # noqa: F401


async def create_tables_for_tests(engine: AsyncEngine) -> None:
    """Test-only helper: creates missing tables in local test DB."""
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

