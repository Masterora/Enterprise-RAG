from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator

from fastapi import APIRouter, Depends, HTTPException, Request, status
from fastapi.responses import StreamingResponse

from enterprise_agent.dependencies import RuntimeDependency, SettingsDependency, authorize
from enterprise_agent.errors import safe_error
from enterprise_agent.models import CheckpointCleanupRequest, RunRequest, RunResult, StreamEvent

router = APIRouter(
    prefix="/internal/v1/runs",
    tags=["runs"],
    dependencies=[Depends(authorize)],
)


@router.post("/invoke")
async def invoke(
    payload: RunRequest,
    runtime: RuntimeDependency,
    settings: SettingsDependency,
) -> RunResult:
    try:
        async with asyncio.timeout(settings.service.request_timeout_seconds):
            return await runtime.invoke(payload)
    except TimeoutError:
        raise HTTPException(
            status_code=status.HTTP_504_GATEWAY_TIMEOUT,
            detail="agent request timeout",
        ) from None


@router.post("/stream")
async def stream(
    request: Request,
    payload: RunRequest,
    runtime: RuntimeDependency,
    settings: SettingsDependency,
) -> StreamingResponse:
    async def events() -> AsyncIterator[bytes]:
        sequence = 0
        try:
            async with asyncio.timeout(settings.service.request_timeout_seconds):
                async for event in runtime.stream(payload):
                    if await request.is_disconnected():
                        return
                    sequence += 1
                    envelope = StreamEvent(sequence=sequence, **event)
                    yield (envelope.model_dump_json() + "\n").encode()
        except Exception as exc:
            sequence += 1
            envelope = StreamEvent(
                sequence=sequence,
                type="error",
                payload={"message": safe_error(exc)},
            )
            yield (envelope.model_dump_json() + "\n").encode()

    return StreamingResponse(events(), media_type="application/x-ndjson")


@router.post("/cleanup")
async def cleanup_checkpoint(
    payload: CheckpointCleanupRequest,
    runtime: RuntimeDependency,
) -> dict[str, bool]:
    await runtime.cleanup_completed_checkpoint(payload.tenant_id, payload.run_id)
    return {"cleaned": True}
