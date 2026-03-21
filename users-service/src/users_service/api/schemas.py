import re
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field, field_validator


class RegisterRequest(BaseModel):
    model_config = ConfigDict(str_strip_whitespace=True)

    login: str = Field(min_length=3, max_length=255)
    password: str = Field(
        min_length=12,
        max_length=128,
        description=(
            "Password policy: 12-128 chars, at least one uppercase letter, "
            "one lowercase letter, one digit, and one special character."
        ),
    )

    @field_validator("password")
    @classmethod
    def validate_password_policy(cls, value: str) -> str:
        if not re.search(r"[A-Z]", value):
            raise ValueError("password must contain an uppercase letter")
        if not re.search(r"[a-z]", value):
            raise ValueError("password must contain a lowercase letter")
        if not re.search(r"[0-9]", value):
            raise ValueError("password must contain a digit")
        if not re.search(r"[^A-Za-z0-9]", value):
            raise ValueError("password must contain a special character")
        return value


class RegisterResponse(BaseModel):
    user_id: UUID
    login: str
    normalized_login: str


class LoginRequest(BaseModel):
    model_config = ConfigDict(str_strip_whitespace=True)

    login: str = Field(min_length=3, max_length=255)
    password: str = Field(min_length=1, max_length=128)


class LoginResponse(BaseModel):
    access_token: str
    user_id: UUID
    normalized_login: str


class VerifyRequest(BaseModel):
    token: str = Field(min_length=1)


class VerifyResponse(BaseModel):
    active: bool
    user_id: UUID | None = None
    normalized_login: str | None = None


class ErrorResponse(BaseModel):
    detail: str
