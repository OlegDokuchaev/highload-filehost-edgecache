from __future__ import annotations

import re
from datetime import UTC, datetime, timedelta
from uuid import UUID

from jose import JWTError, jwt
from passlib.context import CryptContext

from users_service.config import Settings
from users_service.domain.errors import PasswordPolicyError

PASSWORD_REGEX = {
    "uppercase": re.compile(r"[A-Z]"),
    "lowercase": re.compile(r"[a-z]"),
    "digit": re.compile(r"[0-9]"),
    "special": re.compile(r"[^A-Za-z0-9]"),
}


class SecurityService:
    def __init__(self, settings: Settings) -> None:
        self._settings = settings
        self._pwd_context = CryptContext(schemes=["pbkdf2_sha256"], deprecated="auto")

    @staticmethod
    def normalize_login(login: str) -> str:
        return login.strip().lower()

    def validate_password(self, password: str) -> None:
        if not (12 <= len(password) <= 128):
            raise PasswordPolicyError("password length must be between 12 and 128")

        if not PASSWORD_REGEX["uppercase"].search(password):
            raise PasswordPolicyError("password must contain an uppercase letter")
        if not PASSWORD_REGEX["lowercase"].search(password):
            raise PasswordPolicyError("password must contain a lowercase letter")
        if not PASSWORD_REGEX["digit"].search(password):
            raise PasswordPolicyError("password must contain a digit")
        if not PASSWORD_REGEX["special"].search(password):
            raise PasswordPolicyError("password must contain a special character")

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
        except JWTError:
            return None
