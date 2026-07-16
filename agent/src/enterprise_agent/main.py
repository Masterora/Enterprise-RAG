from __future__ import annotations

import os

from fastapi import FastAPI

from enterprise_agent.api.router import api_router
from enterprise_agent.config import Settings, load_settings
from enterprise_agent.errors import register_exception_handlers
from enterprise_agent.lifespan import app_lifespan
from enterprise_agent.observability import configure_telemetry


def create_app(settings: Settings | None = None) -> FastAPI:
    resolved_settings = settings or load_settings()
    application = FastAPI(
        title="Enterprise RAG Agent Service",
        version="5.0.0",
        docs_url=None,
        redoc_url=None,
        openapi_url=None,
        lifespan=app_lifespan,
    )
    application.state.settings = resolved_settings
    application.include_router(api_router)
    register_exception_handlers(application)
    if os.getenv("AGENT_DISABLE_TELEMETRY") != "1":
        configure_telemetry(application, resolved_settings.telemetry)
    return application


app = create_app()
