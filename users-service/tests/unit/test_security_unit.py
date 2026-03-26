from uuid import UUID

import pytest

from users_service.config import Settings
from users_service.domain.errors import PasswordPolicyError
from users_service.services.security import SecurityService


def _security_service() -> SecurityService:
    settings = Settings(
        jwt_secret="unit-secret",
        access_token_expire_minutes=30,
    )
    return SecurityService(settings=settings)


pytestmark = pytest.mark.unit


def test_normalize_login() -> None:
    security = _security_service()
    assert security.normalize_login("  MyLogin  ") == "mylogin"


@pytest.mark.parametrize(
    ("password", "should_raise"),
    [
        ("A1!bcdefghij", False),  # exact min length (12)
        ("A1!" + ("b" * 125), False),  # exact max length (128)
        ("short1A!", True),
        ("A1!" + ("b" * 126), True),  # length 129
        ("alllowercase123!", True),
        ("ALLUPPERCASE123!", True),
        ("NoDigitsPass!!", True),
        ("NoSpecial12345A", True),
        ("ValidPassword!123", False),
    ],
)
def test_password_policy(password: str, should_raise: bool) -> None:
    security = _security_service()
    if should_raise:
        with pytest.raises(PasswordPolicyError):
            security.validate_password(password)
    else:
        security.validate_password(password)


def test_hash_and_verify_password() -> None:
    security = _security_service()
    password = "VeryStrongPass!1"
    password_hash = security.hash_password(password)

    assert password_hash != password
    assert security.verify_password(password, password_hash)
    assert not security.verify_password("WrongPass!1", password_hash)


def test_password_hash_uses_bcrypt_scheme() -> None:
    security = _security_service()
    password_hash = security.hash_password("VeryStrongPass!1")
    assert password_hash.startswith("$argon2")


def test_jwt_create_and_verify_roundtrip() -> None:
    security = _security_service()
    token = security.create_access_token(
        user_id=UUID("db6b4e3b-3a57-4c96-b16d-eb1754f9d4ab"),
        normalized_login="mylogin",
    )
    payload = security.verify_token(token)

    assert payload is not None
    assert payload["normalized_login"] == "mylogin"
    assert payload["user_id"] == "db6b4e3b-3a57-4c96-b16d-eb1754f9d4ab"
