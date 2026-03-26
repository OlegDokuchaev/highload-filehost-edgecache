from __future__ import annotations

from datetime import UTC, datetime, timedelta
from uuid import UUID

import jwt
from jwt import InvalidTokenError
from passlib.context import CryptContext

from users_service.config import Settings
from users_service.domain.password_policy import validate_password_policy


class SecurityService:
    def __init__(self, settings: Settings) -> None:
        self._settings = settings
        self._pwd_context = CryptContext(schemes=["pbkdf2_sha256"], deprecated="auto")

    @staticmethod
    def normalize_login(login: str) -> str:
        return login.strip().lower()

    def validate_password(self, password: str) -> None:
        validate_password_policy(password)

    def hash_password(self, password: str) -> str:
        return self._pwd_context.hash(password)

    def verify_password(self, plain_password: str, password_hash: str) -> bool:
        return self._pwd_context.verify(plain_password, password_hash)

    def create_access_token(self, *, user_id: UUID, normalized_login: str) -> str:
        now = datetime.now(UTC)
        payload: dict[str, str | int] = {
            "sub": str(user_id),
            "normalized_login": normalized_login,
            "iat": int(now.timestamp()),
            "exp": int(
                (
                    now + timedelta(minutes=self._settings.access_token_expire_minutes)
                ).timestamp()
            ),
        }
        return jwt.encode(
            payload,
            self._settings.jwt_secret,
            algorithm=self._settings.jwt_algorithm,
        )

    def verify_token(self, token: str) -> dict[str, str] | None:
        try:
            payload = jwt.decode(
                token,
                self._settings.jwt_secret,
                algorithms=[self._settings.jwt_algorithm],
            )
            user_id = payload.get("sub")
            normalized_login = payload.get("normalized_login")
            if not isinstance(user_id, str) or not isinstance(normalized_login, str):
                return None
            return {"user_id": user_id, "normalized_login": normalized_login}
        except InvalidTokenError:
            return None
