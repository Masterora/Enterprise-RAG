from __future__ import annotations

from typing import Any

import httpx

from enterprise_agent.config import BackendSettings
from enterprise_agent.models import ModelCredentials, RunRequest, ToolResponse


class BackendClient:
    def __init__(self, settings: BackendSettings, service_token: str) -> None:
        self._client = httpx.AsyncClient(
            base_url=settings.base_url.rstrip("/"),
            timeout=settings.timeout_seconds,
            headers={"X-Service-Token": service_token},
            trust_env=False,
        )

    async def close(self) -> None:
        await self._client.aclose()

    async def knowledge_search(
        self,
        request: RunRequest,
        query: str,
    ) -> ToolResponse:
        return await self._post(
            "/internal/v1/tools/knowledge-search",
            {
                **self._context(request),
                "query": request.question,
                "search_query": query,
                "top_k": request.top_k,
                "expected_doc_ids": request.expected_doc_ids,
                "expected_chunk_ids": request.expected_chunk_ids,
                "expected_route": request.expected_route,
            },
        )

    async def knowledge_overview(self, request: RunRequest) -> ToolResponse:
        return await self._post(
            "/internal/v1/tools/knowledge-overview",
            self._context(request),
        )

    async def document_navigation(self, request: RunRequest, topic: str) -> ToolResponse:
        return await self._post(
            "/internal/v1/tools/document-navigation",
            {**self._context(request), "topic": topic},
        )

    async def model_credentials(self, request: RunRequest) -> ModelCredentials:
        response = await self._client.post(
            "/internal/v1/tools/model-credentials",
            json={"tenant_id": request.tenant_id, "user_id": request.user_id},
        )
        response.raise_for_status()
        return ModelCredentials.model_validate(response.json())

    async def _post(self, path: str, payload: dict[str, Any]) -> ToolResponse:
        response = await self._client.post(path, json=payload)
        response.raise_for_status()
        return ToolResponse.model_validate(response.json())

    @staticmethod
    def _context(request: RunRequest) -> dict[str, Any]:
        return {
            "tenant_id": request.tenant_id,
            "user_id": request.user_id,
            "subject_id": request.subject_id,
        }
