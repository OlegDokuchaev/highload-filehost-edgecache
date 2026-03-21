"""Shared password policy used by API validation and SecurityService."""

from __future__ import annotations

import re

from users_service.domain.errors import PasswordPolicyError

PASSWORD_MIN_LENGTH = 12
PASSWORD_MAX_LENGTH = 128

_PASSWORD_CHECKS: tuple[tuple[re.Pattern[str], str], ...] = (
    (re.compile(r"[A-Z]"), "password must contain an uppercase letter"),
    (re.compile(r"[a-z]"), "password must contain a lowercase letter"),
    (re.compile(r"[0-9]"), "password must contain a digit"),
    (re.compile(r"[^A-Za-z0-9]"), "password must contain a special character"),
)


def validate_password_policy(password: str) -> None:
    """Raise PasswordPolicyError if password does not satisfy policy."""
    if not (PASSWORD_MIN_LENGTH <= len(password) <= PASSWORD_MAX_LENGTH):
        raise PasswordPolicyError("password length must be between 12 and 128")

    for pattern, message in _PASSWORD_CHECKS:
        if not pattern.search(password):
            raise PasswordPolicyError(message)
