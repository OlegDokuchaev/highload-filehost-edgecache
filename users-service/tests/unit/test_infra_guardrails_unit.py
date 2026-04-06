from __future__ import annotations

from pathlib import Path

import pytest


pytestmark = pytest.mark.unit


def test_production_src_has_no_create_all_calls() -> None:
    """Guardrail: create_all must stay test-only, never in production src code."""
    src_root = Path(__file__).resolve().parents[2] / "src"
    offenders: list[str] = []
    for py_file in src_root.rglob("*.py"):
        text = py_file.read_text(encoding="utf-8")
        if "metadata.create_all(" in text or ".create_all(" in text:
            offenders.append(str(py_file.relative_to(src_root)))
    assert offenders == []

