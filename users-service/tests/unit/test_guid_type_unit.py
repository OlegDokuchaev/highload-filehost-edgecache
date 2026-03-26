"""Поведение TypeDecorator GUID для PostgreSQL и SQLite."""

from uuid import UUID, uuid4

import pytest
from sqlalchemy.dialects import postgresql, sqlite

from users_service.db.models import GUID


pytestmark = pytest.mark.unit


def test_guid_python_type_is_uuid() -> None:
    assert GUID().python_type is UUID


def test_guid_bind_postgresql_accepts_uuid_and_str() -> None:
    guid = GUID()
    d = postgresql.dialect()
    u = uuid4()
    assert guid.process_bind_param(u, d) is u
    bound = guid.process_bind_param(str(u), d)
    assert isinstance(bound, UUID)
    assert bound == u


def test_guid_bind_sqlite_returns_canonical_str() -> None:
    guid = GUID()
    d = sqlite.dialect()
    u = uuid4()
    assert guid.process_bind_param(u, d) == str(u)
    assert guid.process_bind_param(str(u), d) == str(u)


def test_guid_process_result_value_round_trip() -> None:
    guid = GUID()
    d_pg = postgresql.dialect()
    d_sq = sqlite.dialect()
    u = uuid4()
    assert guid.process_result_value(str(u), d_sq) == u
    assert guid.process_result_value(str(u), d_pg) == u
    assert guid.process_result_value(u, d_pg) == u


def test_guid_process_bind_param_none() -> None:
    guid = GUID()
    assert guid.process_bind_param(None, postgresql.dialect()) is None
    assert guid.process_bind_param(None, sqlite.dialect()) is None


def test_guid_process_result_value_none() -> None:
    guid = GUID()
    assert guid.process_result_value(None, sqlite.dialect()) is None
