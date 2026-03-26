from __future__ import annotations

from typing import cast

from sqlalchemy.exc import IntegrityError, SQLAlchemyError
from sqlalchemy import select, update
from sqlalchemy.engine import CursorResult
from sqlalchemy.ext.asyncio import AsyncSession

from users_service.db.models import User
from users_service.domain.errors import RepositoryError, UniqueConstraintViolationError


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

    async def update_password_hash(
        self,
        *,
        user_id: object,
        expected_password_hash: str,
        new_password_hash: str,
    ) -> int:
        """
        Atomically set new_password_hash only if the row still has expected_password_hash.

        Avoids reverting a concurrent password change (compare-and-swap). Returns affected
        row count (0 if another writer updated the hash first).
        """
        stmt = (
            update(User)
            .where(
                User.id == user_id,
                User.password_hash == expected_password_hash,
            )
            .values(password_hash=new_password_hash)
        )
        try:
            result = cast(
                CursorResult[object],
                await self._session.execute(stmt),
            )
        except IntegrityError as exc:
            raise UniqueConstraintViolationError("unique constraint violation") from exc
        except SQLAlchemyError as exc:
            raise RepositoryError("database error while updating password hash") from exc
        return int(result.rowcount or 0)
