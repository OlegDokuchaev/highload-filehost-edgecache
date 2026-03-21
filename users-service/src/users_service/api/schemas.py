from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field


class RegisterRequest(BaseModel):
    model_config = ConfigDict(str_strip_whitespace=True)

    login: str = Field(min_length=3, max_length=255)
    password: str = Field(min_length=12, max_length=128)


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
