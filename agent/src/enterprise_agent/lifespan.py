from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI

from enterprise_agent.backend_client import BackendClient
from enterprise_agent.config import Settings
from enterprise_agent.graph import AgentRuntime
from enterprise_agent.model_provider import ModelProvider
from enterprise_agent.persistence import open_checkpointer


@asynccontextmanager
async def app_lifespan(app: FastAPI) -> AsyncIterator[None]:
    settings: Settings = app.state.settings
    backend = BackendClient(settings.backend, settings.service.service_token)
    models = ModelProvider(settings.model)
    try:
        async with open_checkpointer(settings.checkpoint) as checkpointer:
            app.state.checkpointer = checkpointer
            app.state.runtime = AgentRuntime(settings.agent, backend, models, checkpointer)
            yield
    finally:
        await backend.close()
        await models.close()
