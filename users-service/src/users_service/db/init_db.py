from sqlalchemy.ext.asyncio import AsyncEngine

from users_service.db.base import Base
from users_service.db import models  # noqa: F401


async def create_tables(engine: AsyncEngine) -> None:
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
