from __future__ import annotations

import logging

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

logger = logging.getLogger(__name__)


def safe_error(error: Exception) -> str:
    if isinstance(error, TimeoutError):
        return "agent request timeout"
    if isinstance(error, ValueError):
        return str(error)
    return "agent execution failed"


def register_exception_handlers(app: FastAPI) -> None:
    @app.exception_handler(Exception)
    async def unhandled_exception(_: Request, exc: Exception) -> JSONResponse:
        logger.exception("unhandled agent request failure", exc_info=exc)
        return JSONResponse(status_code=500, content={"detail": safe_error(exc)})
