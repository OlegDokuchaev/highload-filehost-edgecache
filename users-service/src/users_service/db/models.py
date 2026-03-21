from __future__ import annotations

from datetime import datetime, timezone
from typing import Any, cast
from uuid import UUID, uuid4

from sqlalchemy import CHAR, DateTime, String, TypeDecorator, func
from sqlalchemy.dialects.postgresql import UUID as PG_UUID
from sqlalchemy.engine import Dialect
from sqlalchemy.orm import Mapped, mapped_column
from sqlalchemy.sql.type_api import TypeEngine

from users_service.db.base import Base


class GUID(TypeDecorator[UUID]):
    """Portable UUID type: native UUID on PostgreSQL, CHAR(36) elsewhere."""

    impl = CHAR
    cache_ok = True

    @property
    def python_type(self) -> type[UUID]:
        """Ожидаемый Python-тип значения колонки (не наследуем str от CHAR)."""
        return UUID

    def load_dialect_impl(self, dialect: Dialect) -> TypeEngine[object]:
        if dialect.name == "postgresql":
            return cast(TypeEngine[object], dialect.type_descriptor(PG_UUID(as_uuid=True)))
        return cast(TypeEngine[object], dialect.type_descriptor(CHAR(36)))

    def process_bind_param(
        self,
        value: Any | None,
        dialect: Dialect,
    ) -> str | UUID | None:
        if value is None:
            return None
        if dialect.name == "postgresql":
            return value if isinstance(value, UUID) else UUID(str(value))
        return str(value if isinstance(value, UUID) else UUID(str(value)))

    def process_result_value(
        self,
        value: Any | None,
        dialect: Dialect,
    ) -> UUID | None:
        if value is None:
            return value
        return UUID(str(value))


class User(Base):
    __tablename__ = "users"

    id: Mapped[UUID] = mapped_column(GUID(), primary_key=True, default=uuid4)
    login: Mapped[str] = mapped_column(String(255), nullable=False)
    normalized_login: Mapped[str] = mapped_column(
        String(255),
        nullable=False,
        unique=True,
        index=True,
    )
    password_hash: Mapped[str] = mapped_column(String(255), nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        nullable=False,
        server_default=func.now(),
        default=lambda: datetime.now(timezone.utc),
    )
