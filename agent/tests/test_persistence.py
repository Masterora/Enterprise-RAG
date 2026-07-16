from __future__ import annotations

from typing import Any

import pytest

from enterprise_agent.config import CheckpointSettings
from enterprise_agent.persistence import _schema_dsn, open_checkpointer


class FakeCheckpointer:
    def __init__(self) -> None:
        self.setup_calls = 0

    async def setup(self) -> None:
        self.setup_calls += 1


class FakeContext:
    def __init__(self, checkpointer: FakeCheckpointer) -> None:
        self.checkpointer = checkpointer
        self.closed = False

    async def __aenter__(self) -> FakeCheckpointer:
        return self.checkpointer

    async def __aexit__(self, *_: Any) -> None:
        self.closed = True


@pytest.mark.asyncio
@pytest.mark.parametrize("setup_on_start, expected_calls", [(True, 1), (False, 0)])
async def test_checkpointer_lifecycle(
    monkeypatch: pytest.MonkeyPatch,
    setup_on_start: bool,
    expected_calls: int,
) -> None:
    checkpointer = FakeCheckpointer()
    context = FakeContext(checkpointer)

    class Factory:
        @staticmethod
        def from_conn_string(dsn: str) -> FakeContext:
            assert dsn == "postgres://test?options=-csearch_path%3Dlanggraph"
            return context

    monkeypatch.setattr("enterprise_agent.persistence.AsyncPostgresSaver", Factory)

    async def ensure_schema(_: CheckpointSettings) -> None:
        return None

    monkeypatch.setattr("enterprise_agent.persistence._ensure_checkpoint_schema", ensure_schema)
    settings = CheckpointSettings(
        postgres_dsn="postgres://test",
        setup_on_start=setup_on_start,
    )

    async with open_checkpointer(settings) as opened:
        assert opened is checkpointer
        assert context.closed is False

    assert checkpointer.setup_calls == expected_calls
    assert context.closed is True


def test_schema_dsn_preserves_existing_parameters() -> None:
    settings = CheckpointSettings(
        postgres_dsn="postgresql://localhost/rag?sslmode=disable",
        schema_name="langgraph",
    )
    assert _schema_dsn(settings) == (
        "postgresql://localhost/rag?sslmode=disable&options=-csearch_path%3Dlanggraph"
    )
