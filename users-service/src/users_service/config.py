from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    app_host: str = Field(default="0.0.0.0", alias="USERS_APP_HOST")
    app_port: int = Field(default=8001, alias="USERS_APP_PORT")
    db_url: str = Field(
        default="postgresql+asyncpg://users:users@postgres:5432/users_db",
        alias="USERS_DB_URL",
    )
    jwt_secret: str = Field(default="change-me-please", alias="USERS_JWT_SECRET")
    jwt_algorithm: str = Field(default="HS256", alias="USERS_JWT_ALGORITHM")
    access_token_expire_minutes: int = Field(
        default=60,
        alias="USERS_ACCESS_TOKEN_EXPIRE_MINUTES",
    )

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )
