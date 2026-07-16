from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Any
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver
from psycopg import AsyncConnection, sql

from enterprise_agent.config import CheckpointSettings


@asynccontextmanager
async def open_checkpointer(settings: CheckpointSettings) -> AsyncIterator[Any]:
    """Open the durable LangGraph checkpointer for the application lifespan."""
    if settings.setup_on_start:
        await _ensure_checkpoint_schema(settings)
    async with AsyncPostgresSaver.from_conn_string(_schema_dsn(settings)) as checkpointer:
        if settings.setup_on_start:
            await checkpointer.setup()
        yield checkpointer


async def _ensure_checkpoint_schema(settings: CheckpointSettings) -> None:
    async with await AsyncConnection.connect(settings.postgres_dsn, autocommit=True) as connection:
        await connection.execute(
            sql.SQL("CREATE SCHEMA IF NOT EXISTS {}").format(sql.Identifier(settings.schema_name))
        )


def _schema_dsn(settings: CheckpointSettings) -> str:
    option = f"-csearch_path={settings.schema_name}"
    if "://" not in settings.postgres_dsn:
        return f"{settings.postgres_dsn} options='{option}'"
    parsed = urlsplit(settings.postgres_dsn)
    query = parse_qsl(parsed.query, keep_blank_values=True)
    query = [(key, value) for key, value in query if key != "options"]
    query.append(("options", option))
    return urlunsplit(
        (parsed.scheme, parsed.netloc, parsed.path, urlencode(query), parsed.fragment)
    )


async def check_ready(checkpointer: Any) -> None:
    """Verify that the checkpointer's live PostgreSQL connection is usable."""
    async with checkpointer.lock:
        async with checkpointer.conn.cursor() as cursor:
            await cursor.execute("SELECT 1")
