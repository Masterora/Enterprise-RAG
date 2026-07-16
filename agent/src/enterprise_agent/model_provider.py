from __future__ import annotations

import json
import re
import time
from collections import OrderedDict
from collections.abc import Iterator
from contextlib import contextmanager
from contextvars import ContextVar
from hashlib import sha256
from typing import Any
from urllib.parse import urlparse

import httpx
from langchain_core.messages import AIMessage, BaseMessage, HumanMessage, SystemMessage
from langchain_openai import ChatOpenAI
from pydantic import BaseModel, SecretStr

from enterprise_agent.config import ModelSettings
from enterprise_agent.models import (
    CitationValidation,
    EvidenceEvaluation,
    ExternalLink,
    ModelCredentials,
    RouteDecision,
)
from enterprise_agent.observability import observe_model
from enterprise_agent.prompts import (
    CITATION_VALIDATION_PROMPT,
    EVIDENCE_EVALUATION_PROMPT,
    OVERVIEW_CITATION_VALIDATION_PROMPT,
    REWRITE_PROMPT,
    ROUTE_PROMPT,
    WEB_SEARCH_PROMPT,
)

MODEL_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:/-]{0,159}$")
MAX_MODEL_CLIENTS = 16


class ModelProvider:
    def __init__(self, settings: ModelSettings) -> None:
        self._settings = settings
        self._http = httpx.AsyncClient(timeout=settings.timeout_seconds)
        self._models: OrderedDict[tuple[str, str, str, str], ChatOpenAI] = OrderedDict()
        self._credentials: ContextVar[ModelCredentials | None] = ContextVar(
            "model_credentials", default=None
        )

    async def close(self) -> None:
        await self._http.aclose()

    def validate_provider(self, requested_provider: str) -> None:
        credentials = self._active_credentials()
        requested = requested_provider.strip().lower()
        if requested and requested != credentials.provider:
            raise ValueError("requested model provider is not configured")

    @contextmanager
    def use_credentials(self, credentials: ModelCredentials) -> Iterator[None]:
        token = self._credentials.set(credentials)
        try:
            yield
        finally:
            self._credentials.reset(token)

    async def route(self, question: str, model_name: str) -> RouteDecision:
        response = await self._invoke(
            "agent_route",
            model_name,
            [HumanMessage(content=ROUTE_PROMPT.format(question=question))],
        )
        return _parse_route(_message_text(response), question)

    async def rewrite(self, question: str, search_query: str, model_name: str) -> str:
        response = await self._invoke(
            "agent_rewrite",
            model_name,
            [
                HumanMessage(
                    content=REWRITE_PROMPT.format(
                        question=question,
                        search_query=search_query,
                    )
                )
            ],
        )
        rewritten = " ".join(_message_text(response).strip("`\"'“”").split())
        return rewritten[:500] or question

    async def evaluate_evidence(
        self,
        question: str,
        context: str,
        model_name: str,
        can_rewrite: bool,
    ) -> EvidenceEvaluation:
        response = await self._invoke(
            "evidence_evaluation",
            model_name,
            [
                HumanMessage(
                    content=EVIDENCE_EVALUATION_PROMPT.format(
                        question=question,
                        context=context,
                        can_rewrite="是" if can_rewrite else "否",
                    )
                )
            ],
        )
        fallback = EvidenceEvaluation(
            decision="rewrite" if can_rewrite else "refuse",
            reason="evaluation_parse_failed",
        )
        return _parse_strict_json(_message_text(response), EvidenceEvaluation, fallback)

    async def validate_answer(
        self,
        question: str,
        answer: str,
        context: str,
        model_name: str,
        overview: bool = False,
    ) -> CitationValidation:
        prompt_template = (
            OVERVIEW_CITATION_VALIDATION_PROMPT if overview else CITATION_VALIDATION_PROMPT
        )
        response = await self._invoke(
            "citation_validation",
            model_name,
            [
                HumanMessage(
                    content=prompt_template.format(
                        question=question,
                        answer=answer,
                        context=context,
                    )
                )
            ],
        )
        return _parse_strict_json(
            _message_text(response),
            CitationValidation,
            CitationValidation(supported=False, reason="validation_parse_failed"),
        )

    async def answer(
        self,
        model_name: str,
        prompt: str,
        stream: bool,
        on_delta: Any,
    ) -> str:
        credentials = self._active_credentials()
        model = self._model(model_name)
        messages = [SystemMessage(content="只根据提供的资料回答。"), HumanMessage(content=prompt)]
        started_at = time.perf_counter()
        if not stream:
            try:
                response = await model.ainvoke(messages)
            except Exception:
                observe_model(
                    "agent_answer",
                    "error",
                    time.perf_counter() - started_at,
                    provider=credentials.provider,
                )
                raise
            input_tokens, output_tokens = _usage(response)
            observe_model(
                "agent_answer",
                "success",
                time.perf_counter() - started_at,
                input_tokens,
                output_tokens,
                provider=credentials.provider,
            )
            return _message_text(response)

        chunks: list[str] = []
        input_tokens = 0
        output_tokens = 0
        try:
            async for chunk in model.astream(messages):
                text = _message_text(chunk)
                if text:
                    chunks.append(text)
                    on_delta(text)
                current_input, current_output = _usage(chunk)
                input_tokens = max(input_tokens, current_input)
                output_tokens = max(output_tokens, current_output)
        except Exception:
            observe_model(
                "agent_answer",
                "error",
                time.perf_counter() - started_at,
                provider=credentials.provider,
            )
            raise
        observe_model(
            "agent_answer",
            "success",
            time.perf_counter() - started_at,
            input_tokens,
            output_tokens,
            provider=credentials.provider,
        )
        return "".join(chunks)

    async def search_web(self, question: str, model_name: str) -> list[ExternalLink]:
        credentials = self._active_credentials()
        if credentials.provider != "openrouter":
            raise ValueError("web search requires the OpenRouter provider")
        model = self._validated_model(model_name)
        payload = {
            "model": model,
            "messages": [{"role": "user", "content": WEB_SEARCH_PROMPT.format(question=question)}],
            "max_tokens": 1000,
            "tools": [{"type": "openrouter:web_search"}],
        }
        started_at = time.perf_counter()
        response = await self._http.post(
            credentials.base_url.rstrip("/") + "/chat/completions",
            headers={
                "Authorization": f"Bearer {credentials.api_key}",
                "Content-Type": "application/json",
            },
            json=payload,
        )
        if response.is_error:
            observe_model(
                "web_search",
                "error",
                time.perf_counter() - started_at,
                provider=credentials.provider,
            )
            response.raise_for_status()
        body = response.json()
        usage = body.get("usage") or {}
        observe_model(
            "web_search",
            "success",
            time.perf_counter() - started_at,
            int(usage.get("prompt_tokens") or 0),
            int(usage.get("completion_tokens") or 0),
            float(usage.get("cost") or 0),
            provider=credentials.provider,
        )
        choices = body.get("choices") or []
        if not choices:
            return []
        message = choices[0].get("message") or {}
        return _external_links(message.get("content") or "", message.get("annotations") or [])

    async def _invoke(
        self,
        operation: str,
        model_name: str,
        messages: list[BaseMessage],
    ) -> AIMessage:
        credentials = self._active_credentials()
        started_at = time.perf_counter()
        try:
            response = await self._model(model_name).ainvoke(messages)
        except Exception:
            observe_model(
                operation,
                "error",
                time.perf_counter() - started_at,
                provider=credentials.provider,
            )
            raise
        input_tokens, output_tokens = _usage(response)
        observe_model(
            operation,
            "success",
            time.perf_counter() - started_at,
            input_tokens,
            output_tokens,
            provider=credentials.provider,
        )
        return response

    def _model(self, model_name: str) -> ChatOpenAI:
        credentials = self._active_credentials()
        model = self._validated_model(model_name)
        fingerprint = sha256(credentials.api_key.encode()).hexdigest()[:16]
        cache_key = (credentials.provider, credentials.base_url.rstrip("/"), fingerprint, model)
        cached = self._models.get(cache_key)
        if cached is not None:
            self._models.move_to_end(cache_key)
            return cached
        client = ChatOpenAI(
            model=model,
            api_key=SecretStr(credentials.api_key),
            base_url=credentials.base_url,
            timeout=self._settings.timeout_seconds,
            max_retries=self._settings.max_retries,
            max_completion_tokens=self._settings.max_output_tokens,
            stream_usage=True,
            http_async_client=self._http,
        )
        self._models[cache_key] = client
        if len(self._models) > MAX_MODEL_CLIENTS:
            self._models.popitem(last=False)
        return client

    def _active_credentials(self) -> ModelCredentials:
        credentials = self._credentials.get()
        if credentials is not None:
            return credentials
        return ModelCredentials(
            provider=self._settings.provider,
            base_url=self._settings.base_url,
            api_key=self._settings.api_key,
        )

    def _validated_model(self, model_name: str) -> str:
        value = model_name.strip() or self._settings.default_model
        if not MODEL_PATTERN.fullmatch(value):
            raise ValueError("invalid model identifier")
        return value


def _message_text(message: Any) -> str:
    content = getattr(message, "content", "")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "".join(
            str(item.get("text", "")) if isinstance(item, dict) else str(item) for item in content
        )
    return str(content or "")


def _usage(message: Any) -> tuple[int, int]:
    usage = getattr(message, "usage_metadata", None) or {}
    return int(usage.get("input_tokens") or 0), int(usage.get("output_tokens") or 0)


def _parse_route(raw: str, question: str) -> RouteDecision:
    cleaned = raw.strip().removeprefix("```json").removeprefix("```").removesuffix("```").strip()
    try:
        decision = RouteDecision.model_validate_json(cleaned)
    except Exception:
        return RouteDecision(route="rag", search_query=question, reason="route_parse_failed")
    if not decision.search_query:
        decision.search_query = question
    if not decision.topic:
        decision.topic = question
    return decision


def _parse_strict_json[ModelT: BaseModel](
    raw: str, model_type: type[ModelT], fallback: ModelT
) -> ModelT:
    cleaned = raw.strip().removeprefix("```json").removeprefix("```").removesuffix("```").strip()
    try:
        return model_type.model_validate_json(cleaned)
    except Exception:
        return fallback


def _external_links(content: str, annotations: list[dict[str, Any]]) -> list[ExternalLink]:
    candidates: list[ExternalLink] = []
    cleaned = (
        content.strip().removeprefix("```json").removeprefix("```").removesuffix("```").strip()
    )
    try:
        for item in json.loads(cleaned):
            candidates.append(ExternalLink.model_validate(item))
    except (json.JSONDecodeError, TypeError, ValueError):
        pass
    for annotation in annotations:
        citation = annotation.get("url_citation") or {}
        if annotation.get("type") == "url_citation" and citation.get("url"):
            candidates.append(
                ExternalLink(
                    title=citation.get("title") or citation["url"],
                    url=citation["url"],
                )
            )

    result: list[ExternalLink] = []
    seen: set[str] = set()
    for link in candidates:
        parsed = urlparse(link.url)
        hostname = (parsed.hostname or "").lower()
        if parsed.scheme not in {"http", "https"} or "." not in hostname or hostname == "www":
            continue
        if link.url in seen:
            continue
        seen.add(link.url)
        result.append(link)
        if len(result) == 5:
            break
    return result
