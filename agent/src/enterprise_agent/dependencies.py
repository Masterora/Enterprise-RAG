from __future__ import annotations

import hmac
from typing import Annotated, cast

from fastapi import Depends, Header, HTTPException, Request, status

from enterprise_agent.config import Settings
from enterprise_agent.graph import AgentRuntime


def get_settings(request: Request) -> Settings:
    return cast(Settings, request.app.state.settings)


def get_runtime(request: Request) -> AgentRuntime:
    return cast(AgentRuntime, request.app.state.runtime)


def authorize(
    settings: Annotated[Settings, Depends(get_settings)],
    service_token: Annotated[str | None, Header(alias="X-Service-Token")] = None,
) -> None:
    if service_token is None or not hmac.compare_digest(
        service_token,
        settings.service.service_token,
    ):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="unauthorized")


SettingsDependency = Annotated[Settings, Depends(get_settings)]
RuntimeDependency = Annotated[AgentRuntime, Depends(get_runtime)]
