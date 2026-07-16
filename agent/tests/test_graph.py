from __future__ import annotations

import asyncio
from collections.abc import Iterator
from contextlib import contextmanager
from typing import Any

import pytest
from langgraph.checkpoint.memory import InMemorySaver

from enterprise_agent.config import AgentSettings
from enterprise_agent.graph import AgentRuntime
from enterprise_agent.models import (
    CitationValidation,
    EvidenceEvaluation,
    ExternalLink,
    ModelCredentials,
    RetrievalChunk,
    RetrievalMetrics,
    RouteDecision,
    RunRequest,
    ToolCoverage,
    ToolResponse,
)


def request(run_id: str = "run-1", question: str = "报销时限是多少？") -> RunRequest:
    return RunRequest(
        run_id=run_id,
        tenant_id="tenant-1",
        session_id="shared-session",
        message_id="message-1",
        user_id="user-1",
        subject_id="subject-1",
        question=question,
    )


def chunk(index: int = 1) -> RetrievalChunk:
    return RetrievalChunk(
        id=f"chunk-{index}",
        doc_id="doc-1",
        doc_name="报销制度.md",
        content="报销申请应在费用发生后十个工作日内提交。",
    )


def test_tool_response_rejects_null_collections() -> None:
    with pytest.raises(ValueError):
        ToolResponse.model_validate({"chunks": None, "stages": None})


class FakeBackend:
    def __init__(self, search_responses: list[ToolResponse] | None = None) -> None:
        self.search_responses = search_responses or []
        self.search_queries: list[str] = []

    async def model_credentials(self, _: RunRequest) -> ModelCredentials:
        return ModelCredentials(
            provider="openrouter",
            base_url="https://example.com/v1",
            api_key="tenant-test-key",
        )

    async def knowledge_search(self, _: RunRequest, query: str) -> ToolResponse:
        self.search_queries.append(query)
        return self.search_responses.pop(0)

    async def knowledge_overview(self, _: RunRequest) -> ToolResponse:
        return ToolResponse(
            content="知识库包含报销制度。",
            chunks=[chunk()],
            coverage=ToolCoverage(total_documents=1, covered_documents=1, complete=True),
        )

    async def document_navigation(self, _: RunRequest, topic: str) -> ToolResponse:
        return ToolResponse(content=f"与{topic}相关的文档。", chunks=[chunk()])


class FakeModels:
    def __init__(
        self,
        route: str = "rag",
        answer: str = "十个工作日内。[引用1]",
        evaluations: list[EvidenceEvaluation] | None = None,
        validations: list[CitationValidation] | None = None,
    ) -> None:
        self.route_name = route
        self.answer_text = answer
        self.evaluations = evaluations or [EvidenceEvaluation(decision="sufficient")]
        self.validations = validations or [CitationValidation(supported=True)]
        self.rewrite_calls = 0
        self.evaluation_calls = 0
        self.answer_calls = 0
        self.answer_prompts: list[str] = []
        self.validation_modes: list[bool] = []

    def validate_provider(self, _: str) -> None:
        return None

    @contextmanager
    def use_credentials(self, _: ModelCredentials) -> Iterator[None]:
        yield

    async def route(self, question: str, _: str) -> RouteDecision:
        return RouteDecision(
            route=self.route_name,  # type: ignore[arg-type]
            search_query=question,
            topic="报销",
            reason="test",
        )

    async def rewrite(self, _: str, __: str, ___: str) -> str:
        self.rewrite_calls += 1
        return "费用报销申请提交时限"

    async def evaluate_evidence(self, _: str, __: str, ___: str, ____: bool) -> EvidenceEvaluation:
        self.evaluation_calls += 1
        if len(self.evaluations) > 1:
            return self.evaluations.pop(0)
        return self.evaluations[0]

    async def validate_answer(
        self, _: str, __: str, ___: str, ____: str, overview: bool = False
    ) -> CitationValidation:
        self.validation_modes.append(overview)
        if len(self.validations) > 1:
            return self.validations.pop(0)
        return self.validations[0]

    async def answer(self, _: str, prompt: str, __: bool, ___: Any) -> str:
        self.answer_calls += 1
        self.answer_prompts.append(prompt)
        return self.answer_text

    async def search_web(self, _: str, __: str) -> list[ExternalLink]:
        return []


@pytest.mark.asyncio
async def test_rag_graph_completes_and_persists_by_run_id() -> None:
    backend = FakeBackend(
        [
            ToolResponse(
                chunks=[chunk()],
                metrics=RetrievalMetrics(returned_count=1),
                stages=["retrieval.done"],
            )
        ]
    )
    checkpointer = InMemorySaver()
    runtime = AgentRuntime(AgentSettings(), backend, FakeModels(), checkpointer)

    result = await runtime.invoke(request())

    assert result.answer == "十个工作日内。[引用1]"
    assert [step.kind for step in result.agent_steps] == [
        "planning",
        "tool",
        "observation",
        "synthesis",
        "validation",
    ]
    assert result.metrics.search_query == "报销时限是多少？"
    assert result.metrics.citation_count == 1
    stored = await checkpointer.aget_tuple({"configurable": {"thread_id": "tenant-1:run-1"}})
    assert stored is not None
    await runtime.cleanup_completed_checkpoint("tenant-1", "run-1")
    assert await checkpointer.aget_tuple({"configurable": {"thread_id": "tenant-1:run-1"}}) is None
    session_scoped = await checkpointer.aget_tuple(
        {"configurable": {"thread_id": "shared-session"}}
    )
    assert session_scoped is None


@pytest.mark.asyncio
async def test_resume_waits_for_cancelled_stream_to_release_run() -> None:
    class SerialGraph:
        def __init__(self) -> None:
            self.active = 0
            self.max_active = 0
            self.stream_started = asyncio.Event()

        async def astream(self, *_: Any, **__: Any) -> Any:
            self.active += 1
            self.max_active = max(self.max_active, self.active)
            self.stream_started.set()
            try:
                await asyncio.Future()
                if False:
                    yield {}
            finally:
                await asyncio.sleep(0.02)
                self.active -= 1

        async def ainvoke(self, initial: dict[str, Any], _: Any) -> dict[str, Any]:
            self.active += 1
            self.max_active = max(self.max_active, self.active)
            try:
                return {**initial, "answer": "恢复成功"}
            finally:
                self.active -= 1

    runtime = AgentRuntime(AgentSettings(), FakeBackend(), FakeModels())
    graph = SerialGraph()
    runtime._graph = graph

    async def consume_stream() -> None:
        async for _ in runtime.stream(request()):
            pass

    stream_task = asyncio.create_task(consume_stream())
    await graph.stream_started.wait()
    stream_task.cancel()
    resume_task = asyncio.create_task(runtime.invoke(request()))

    with pytest.raises(asyncio.CancelledError):
        await stream_task
    result = await resume_task

    assert result.answer == "恢复成功"
    assert graph.max_active == 1


@pytest.mark.asyncio
async def test_cancelled_langgraph_run_can_resume_from_checkpoint() -> None:
    class CancellableModels(FakeModels):
        def __init__(self) -> None:
            super().__init__()
            self.route_started = asyncio.Event()
            self.route_calls = 0

        async def route(self, question: str, model: str) -> RouteDecision:
            self.route_calls += 1
            if self.route_calls == 1:
                self.route_started.set()
                await asyncio.Future()
            return await super().route(question, model)

    backend = FakeBackend([ToolResponse(chunks=[chunk()])])
    models = CancellableModels()
    runtime = AgentRuntime(AgentSettings(), backend, models, InMemorySaver())

    async def consume_stream() -> None:
        async for _ in runtime.stream(request()):
            pass

    stream_task = asyncio.create_task(consume_stream())
    await models.route_started.wait()
    stream_task.cancel()
    with pytest.raises(asyncio.CancelledError):
        await stream_task

    result = await runtime.invoke(request())

    assert result.answer == "十个工作日内。[引用1]"
    assert models.route_calls == 2


@pytest.mark.asyncio
async def test_empty_retrieval_rewrites_once_then_returns_no_answer() -> None:
    backend = FakeBackend([ToolResponse(), ToolResponse()])
    models = FakeModels()
    runtime = AgentRuntime(AgentSettings(max_rewrite_attempts=1), backend, models)

    result = await runtime.invoke(request())

    assert backend.search_queries == ["报销时限是多少？", "费用报销申请提交时限"]
    assert models.rewrite_calls == 1
    assert models.answer_calls == 0
    assert result.answer == "无法确定"
    assert result.metrics.query_rewritten is True
    assert result.metrics.search_query == "费用报销申请提交时限"
    assert result.metrics.evaluation_passed is False
    assert result.agent_steps[-1].iteration == 2


@pytest.mark.asyncio
async def test_partial_but_useful_evidence_is_answered_with_scope() -> None:
    backend = FakeBackend([ToolResponse(chunks=[chunk()])])
    models = FakeModels(
        answer="可先按规范中的基本结构编写。[引用1]",
        evaluations=[
            EvidenceEvaluation(
                decision="sufficient",
                reason="资料足以说明基本用法",
                missing_aspects=["完整进阶教程"],
            )
        ],
    )
    runtime = AgentRuntime(AgentSettings(), backend, models)

    result = await runtime.invoke(request(question="这个格式怎么用？"))

    assert result.metrics.answered is True
    assert result.metrics.evaluation_passed is True
    assert "资料足以说明基本用法；未覆盖内容：完整进阶教程" in models.answer_prompts[0]


@pytest.mark.asyncio
@pytest.mark.parametrize("route", ["overview", "navigation"])
async def test_knowledge_routes_skip_rag_evaluation(route: str) -> None:
    backend = FakeBackend()
    models = FakeModels(route=route)
    runtime = AgentRuntime(AgentSettings(), backend, models)

    result = await runtime.invoke(request())

    assert result.answer == "十个工作日内。[引用1]"
    assert result.metrics.route == route
    assert [step.kind for step in result.agent_steps] == [
        "planning",
        "tool",
        "synthesis",
        "validation",
    ]
    assert models.evaluation_calls == 0
    assert models.validation_modes == [route == "overview"]


@pytest.mark.asyncio
async def test_incomplete_overview_is_refused_without_answer_generation() -> None:
    class IncompleteOverviewBackend(FakeBackend):
        async def knowledge_overview(self, _: RunRequest) -> ToolResponse:
            return ToolResponse(
                content="知识库共两篇文档，目前只覆盖一篇。",
                chunks=[chunk()],
                coverage=ToolCoverage(
                    total_documents=2,
                    covered_documents=1,
                    complete=False,
                ),
            )

    models = FakeModels(route="overview")
    runtime = AgentRuntime(AgentSettings(), IncompleteOverviewBackend(), models)

    result = await runtime.invoke(request())

    assert result.answer == "无法确定"
    assert result.metrics.answered is False
    assert models.evaluation_calls == 0
    assert models.answer_calls == 0
    assert result.agent_steps[-1].detail == "知识概览仅覆盖1/2篇文档，无法可靠回答"


@pytest.mark.asyncio
async def test_citations_are_bounded_by_configuration() -> None:
    chunks = [chunk(index) for index in range(1, 5)]
    backend = FakeBackend([ToolResponse(chunks=chunks)])
    models = FakeModels(answer="第一条。[引用1]")
    runtime = AgentRuntime(AgentSettings(max_citations=2), backend, models)

    result = await runtime.invoke(request())

    assert [item.id for item in result.chunks] == ["chunk-1"]
    assert result.answer == "第一条。[引用1]"


@pytest.mark.asyncio
async def test_semantically_insufficient_sources_are_rewritten_then_refused() -> None:
    backend = FakeBackend([ToolResponse(chunks=[chunk()]), ToolResponse(chunks=[chunk()])])
    models = FakeModels(
        evaluations=[
            EvidenceEvaluation(decision="rewrite", rewrite_query="差旅报销提交时限"),
            EvidenceEvaluation(decision="refuse", reason="资料未覆盖差旅场景"),
        ]
    )
    runtime = AgentRuntime(AgentSettings(max_rewrite_attempts=1), backend, models)

    result = await runtime.invoke(request())

    assert backend.search_queries == ["报销时限是多少？", "差旅报销提交时限"]
    assert result.answer == "无法确定"
    assert models.answer_calls == 0
    assert result.agent_steps[-1].kind == "refusal"


@pytest.mark.asyncio
async def test_invalid_citation_is_repaired_once_then_refused() -> None:
    backend = FakeBackend([ToolResponse(chunks=[chunk()])])
    models = FakeModels(answer="十个工作日内。[引用9]")
    runtime = AgentRuntime(AgentSettings(), backend, models)

    result = await runtime.invoke(request())

    assert models.answer_calls == 2
    assert result.answer == "无法确定"
    assert result.chunks == []


@pytest.mark.asyncio
async def test_unsupported_claim_is_repaired_once_then_refused() -> None:
    backend = FakeBackend([ToolResponse(chunks=[chunk()])])
    models = FakeModels(
        validations=[CitationValidation(supported=False, reason="unsupported_claim")]
    )
    runtime = AgentRuntime(AgentSettings(), backend, models)

    result = await runtime.invoke(request())

    assert models.answer_calls == 2
    assert result.answer == "无法确定"
