from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Any

import httpx
import pytest

import enterprise_agent.lifespan as lifespan_module
from enterprise_agent.main import app
from enterprise_agent.models import RunRequest, RunResult, ToolResponse

TOKEN = "0123456789abcdef"
PAYLOAD = {
    "run_id": "run-1",
    "tenant_id": "tenant-1",
    "session_id": "session-1",
    "message_id": "message-1",
    "user_id": "user-1",
    "subject_id": "subject-1",
    "question": "测试问题",
}


class FakeRuntime:
    async def invoke(self, _: RunRequest) -> RunResult:
        return RunResult(answer="测试回答")

    async def stream(self, _: RunRequest) -> AsyncIterator[dict[str, Any]]:
        yield {"type": "delta", "payload": {"content": "测试"}}
        yield {"type": "result", "payload": RunResult(answer="测试").model_dump()}


class ReadyCursor:
    async def execute(self, query: str) -> None:
        assert query == "SELECT 1"

    async def __aenter__(self) -> ReadyCursor:
        return self

    async def __aexit__(self, *_: Any) -> None:
        return None


class ReadyConnection:
    def cursor(self) -> ReadyCursor:
        return ReadyCursor()


class ReadyCheckpointer:
    conn = ReadyConnection()
    lock = asyncio.Lock()


@pytest.fixture
def configured_app() -> Any:
    original_token = app.state.settings.service.service_token
    app.state.settings.service.service_token = TOKEN
    app.state.runtime = FakeRuntime()
    app.state.checkpointer = ReadyCheckpointer()
    yield app
    app.state.settings.service.service_token = original_token


@pytest.mark.asyncio
async def test_health_and_readiness_are_separate(configured_app: Any) -> None:
    transport = httpx.ASGITransport(app=configured_app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        health = await client.get("/health")
        ready = await client.get("/ready")

    assert health.status_code == 200
    assert health.json() == {"status": "ok"}
    assert ready.status_code == 200
    assert ready.json() == {"status": "ready"}


@pytest.mark.asyncio
async def test_readiness_fails_when_postgres_is_unavailable(configured_app: Any) -> None:
    class BrokenCursor(ReadyCursor):
        async def execute(self, _: str) -> None:
            raise RuntimeError("postgres unavailable")

    class BrokenConnection:
        def cursor(self) -> BrokenCursor:
            return BrokenCursor()

    configured_app.state.checkpointer.conn = BrokenConnection()
    transport = httpx.ASGITransport(app=configured_app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/ready")

    assert response.status_code == 503
    assert response.json() == {"detail": "service is not ready"}


def test_internal_response_models_reject_unknown_fields() -> None:
    with pytest.raises(ValueError):
        ToolResponse.model_validate({"content": "ok", "unexpected": True})


@pytest.mark.asyncio
async def test_invoke_requires_service_token(configured_app: Any) -> None:
    transport = httpx.ASGITransport(app=configured_app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        unauthorized = await client.post("/internal/v1/runs/invoke", json=PAYLOAD)
        authorized = await client.post(
            "/internal/v1/runs/invoke",
            json=PAYLOAD,
            headers={"X-Service-Token": TOKEN},
        )

    assert unauthorized.status_code == 401
    assert authorized.status_code == 200
    assert authorized.json()["answer"] == "测试回答"


@pytest.mark.asyncio
async def test_invoke_timeout_cancels_runtime(configured_app: Any) -> None:
    class SlowRuntime:
        cancelled = False

        async def invoke(self, _: RunRequest) -> RunResult:
            try:
                await asyncio.Event().wait()
            except asyncio.CancelledError:
                self.cancelled = True
                raise

    runtime = SlowRuntime()
    previous_timeout = configured_app.state.settings.service.request_timeout_seconds
    previous_runtime = configured_app.state.runtime
    configured_app.state.settings.service.request_timeout_seconds = 1
    configured_app.state.runtime = runtime
    transport = httpx.ASGITransport(app=configured_app)
    try:
        async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
            response = await client.post(
                "/internal/v1/runs/invoke",
                json=PAYLOAD,
                headers={"X-Service-Token": TOKEN},
            )
    finally:
        configured_app.state.settings.service.request_timeout_seconds = previous_timeout
        configured_app.state.runtime = previous_runtime

    assert response.status_code == 504
    assert response.json() == {"detail": "agent request timeout"}
    assert runtime.cancelled is True


@pytest.mark.asyncio
async def test_stream_returns_ordered_ndjson_events(configured_app: Any) -> None:
    transport = httpx.ASGITransport(app=configured_app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/internal/v1/runs/stream",
            json=PAYLOAD,
            headers={"X-Service-Token": TOKEN},
        )

    events = [httpx.Response(200, content=line).json() for line in response.content.splitlines()]
    assert response.headers["content-type"].startswith("application/x-ndjson")
    assert [event["sequence"] for event in events] == [1, 2]
    assert [event["type"] for event in events] == ["delta", "result"]


@pytest.mark.asyncio
async def test_stream_timeout_cancels_runtime(configured_app: Any) -> None:
    class SlowRuntime:
        cancelled = False

        async def stream(self, _: RunRequest) -> AsyncIterator[dict[str, Any]]:
            try:
                await asyncio.Event().wait()
                yield {"type": "result", "payload": RunResult(answer="late").model_dump()}
            except asyncio.CancelledError:
                self.cancelled = True
                raise

    runtime = SlowRuntime()
    previous_timeout = configured_app.state.settings.service.request_timeout_seconds
    previous_runtime = configured_app.state.runtime
    configured_app.state.settings.service.request_timeout_seconds = 1
    configured_app.state.runtime = runtime
    transport = httpx.ASGITransport(app=configured_app)
    try:
        async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
            response = await client.post(
                "/internal/v1/runs/stream",
                json=PAYLOAD,
                headers={"X-Service-Token": TOKEN},
            )
    finally:
        configured_app.state.settings.service.request_timeout_seconds = previous_timeout
        configured_app.state.runtime = previous_runtime

    events = [httpx.Response(200, content=line).json() for line in response.content.splitlines()]
    assert [event["type"] for event in events] == ["error"]
    assert runtime.cancelled is True


@pytest.mark.asyncio
async def test_lifespan_wires_persistence_and_closes_clients(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    resources: list[Any] = []
    checkpointer = object()

    class Resource:
        def __init__(self, *_: Any) -> None:
            self.closed = False
            resources.append(self)

        async def close(self) -> None:
            self.closed = True

    class Runtime:
        def __init__(self, _: Any, backend: Any, models: Any, saver: Any) -> None:
            assert backend is resources[0]
            assert models is resources[1]
            assert saver is checkpointer

    @asynccontextmanager
    async def fake_open_checkpointer(_: Any) -> AsyncIterator[object]:
        yield checkpointer

    monkeypatch.setattr(lifespan_module, "BackendClient", Resource)
    monkeypatch.setattr(lifespan_module, "ModelProvider", Resource)
    monkeypatch.setattr(lifespan_module, "AgentRuntime", Runtime)
    monkeypatch.setattr(lifespan_module, "open_checkpointer", fake_open_checkpointer)

    async with lifespan_module.app_lifespan(app):
        assert isinstance(app.state.runtime, Runtime)
        assert all(resource.closed is False for resource in resources)

    assert all(resource.closed is True for resource in resources)
