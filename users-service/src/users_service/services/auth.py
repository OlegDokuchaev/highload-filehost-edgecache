from __future__ import annotations

import logging
from uuid import UUID

from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from users_service.domain.errors import (
    InvalidCredentialsError,
    UniqueConstraintViolationError,
    UserAlreadyExistsError,
)
from users_service.repositories.users import UserRepository
from users_service.services.security import SecurityService

logger = logging.getLogger(__name__)


class AuthService:
    def __init__(
        self,
        session_factory: async_sessionmaker[AsyncSession],
        security_service: SecurityService,
    ) -> None:
        self._session_factory = session_factory
        self._security = security_service

    async def register(self, *, login: str, password: str) -> tuple[UUID, str, str]:
        normalized_login = self._security.normalize_login(login)
        self._security.validate_password(password)
        password_hash = self._security.hash_password(password)

        async with self._session_factory() as session:
            repo = UserRepository(session)
            existing = await repo.get_by_normalized_login(normalized_login)
            if existing is not None:
                raise UserAlreadyExistsError("normalized_login already exists")

            try:
                user = await repo.create_user(
                    login=login,
                    normalized_login=normalized_login,
                    password_hash=password_hash,
                )
                await session.commit()
                return user.id, user.login, user.normalized_login
            except UniqueConstraintViolationError as exc:
                await session.rollback()
                raise UserAlreadyExistsError("normalized_login already exists") from exc

    async def login(self, *, login: str, password: str) -> tuple[str, UUID, str]:
        normalized_login = self._security.normalize_login(login)

        async with self._session_factory() as session:
            # UserRepository всегда привязан к этому же AsyncSession — commit ниже
            # относится к той же транзакции, что и UPDATE в репозитории.
            repo = UserRepository(session)
            user = await repo.get_by_normalized_login(normalized_login)
            if user is None:
                # ВАЖНО: выравниваем время ответа, чтобы нельзя было отличить
                # "пользователь не найден" от "неверный пароль" по таймингу.
                # Результат игнорируется.
                self._security.verify_password(password, self._security.dummy_password_hash)
                raise InvalidCredentialsError("invalid credentials")
            password_hash_at_read = user.password_hash
            ok, updated_hash = self._security.verify_password_and_update(
                password,
                password_hash_at_read,
            )
            if not ok:
                raise InvalidCredentialsError("invalid credentials")
            if updated_hash is not None:
                rows = await repo.update_password_hash(
                    user_id=user.id,
                    expected_password_hash=password_hash_at_read,
                    new_password_hash=updated_hash,
                )
                if rows == 0:
                    # Параллельный запрос уже сменил пароль — не перезаписываем чужой хеш.
                    logger.info(
                        "password rehash skipped: row not updated "
                        "(concurrent password change or stale read)"
                    )
                await session.commit()

            token = self._security.create_access_token(
                user_id=user.id,
                normalized_login=user.normalized_login,
            )
            return token, user.id, user.normalized_login

    def verify(self, token: str) -> tuple[bool, UUID | None, str | None]:
        verified_payload = self._security.verify_token(token)
        if verified_payload is None:
            return False, None, None
        user_id = verified_payload["user_id"]
        try:
            parsed_user_id = UUID(user_id)
        except ValueError:
            return False, None, None
        return (
            True,
            parsed_user_id,
            verified_payload["normalized_login"],
        )
