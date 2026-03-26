from __future__ import annotations

from sqlalchemy.exc import IntegrityError
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from users_service.db.models import User
from users_service.domain.errors import UniqueConstraintViolationError


class UserRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    async def get_by_normalized_login(self, normalized_login: str) -> User | None:
        stmt = select(User).where(User.normalized_login == normalized_login)
        result = await self._session.execute(stmt)
        return result.scalar_one_or_none()

    async def create_user(
        self,
        *,
        login: str,
        normalized_login: str,
        password_hash: str,
    ) -> User:
        user = User(
            login=login,
            normalized_login=normalized_login,
            password_hash=password_hash,
        )
        self._session.add(user)
        try:
            await self._session.flush()
        except IntegrityError as exc:
            raise UniqueConstraintViolationError("unique constraint violation") from exc
        await self._session.refresh(user)
        return user
