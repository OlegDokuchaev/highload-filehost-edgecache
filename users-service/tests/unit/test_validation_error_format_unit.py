"""Безопасное форматирование ошибок валидации (main.format_validation_error_detail)."""

import pytest

from users_service.main import format_validation_error_detail


pytestmark = pytest.mark.unit


@pytest.mark.parametrize(
    ("errors", "expected_substrings"),
    [
        ([], ["invalid input"]),
        ([{"msg": "first"}, {"msg": "second"}], ["first", "second"]),
        ([{"message": "via message"}], ["via message"]),
        ([{"msg": ""}, {"foo": 1}], ["{"]),  # пустой msg -> str(dict)
        ([{"unexpected": "only"}], ["unexpected"]),
    ],
)
def test_format_validation_error_detail_joins_messages(
    errors: list[object],
    expected_substrings: list[str],
) -> None:
    detail = format_validation_error_detail(errors)
    for part in expected_substrings:
        assert part in detail
    if len(errors) > 1 and all(isinstance(e, dict) and e.get("msg") for e in errors):
        assert "; " in detail


def test_format_validation_error_detail_non_dict_entries() -> None:
    detail = format_validation_error_detail(["plain", 42])
    assert "plain" in detail
    assert "42" in detail


def test_format_validation_error_detail_never_raises() -> None:
    class Bad:
        def __str__(self) -> str:
            raise RuntimeError("boom")

    # внутренний цикл подставляет "invalid input" для сбойного элемента
    detail = format_validation_error_detail([Bad()])
    assert "invalid input" in detail

    detail_top = format_validation_error_detail(object())  # type: ignore[arg-type]
    assert detail_top == "invalid input"
