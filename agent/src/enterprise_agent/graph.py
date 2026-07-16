from __future__ import annotations

import asyncio
import time
from collections.abc import AsyncIterator, Callable
from typing import Any, cast
from uuid import uuid4
from weakref import WeakValueDictionary

from langgraph.config import get_stream_writer
from langgraph.graph import END, START, StateGraph

from enterprise_agent.backend_client import BackendClient
from enterprise_agent.citations import (
    is_no_answer,
    select_referenced_sources,
    validate_reference_structure,
)
from enterprise_agent.config import AgentSettings
from enterprise_agent.model_provider import ModelProvider
from enterprise_agent.models import (
    AgentState,
    AgentStep,
    CitationValidation,
    EvidenceEvaluation,
    ExternalLink,
    RetrievalChunk,
    RetrievalMetrics,
    RunRequest,
    RunResult,
    ToolCoverage,
)
from enterprise_agent.observability import (
    AGENT_DURATION,
    AGENT_IN_FLIGHT,
    AGENT_ITERATIONS,
    AGENT_RUNS,
    AGENT_TRANSITIONS,
    observe_tool,
    traced_node,
)
from enterprise_agent.prompts import (
    ANSWER_PROMPT,
    ANSWER_REPAIR_PROMPT,
    OVERVIEW_ANSWER_PROMPT,
    OVERVIEW_ANSWER_REPAIR_PROMPT,
)


class AgentRuntime:
    def __init__(
        self,
        settings: AgentSettings,
        backend: BackendClient,
        models: ModelProvider,
        checkpointer: Any = None,
    ) -> None:
        self._settings = settings
        self._backend = backend
        self._models = models
        self._checkpointer = checkpointer
        self._run_slots = asyncio.Semaphore(settings.max_concurrent_runs)
        self._run_locks: WeakValueDictionary[str, asyncio.Lock] = WeakValueDictionary()
        self._graph = self._build_graph(checkpointer)

    async def invoke(self, request: RunRequest) -> RunResult:
        state = await self._run(request, stream=False)
        return _result(state)

    async def stream(self, request: RunRequest) -> AsyncIterator[dict[str, Any]]:
        self._validate_question(request)
        initial = _initial_state(request)
        initial["stream"] = True
        config = _run_config(request)
        latest = initial
        async with self._lock_for(request):
            async with self._run_slots:
                AGENT_IN_FLIGHT.inc()
                started_at = time.perf_counter()
                status = "error"
                try:
                    credentials = await self._backend.model_credentials(request)
                    with self._models.use_credentials(credentials):
                        self._models.validate_provider(request.llm_provider)
                        async for part in self._graph.astream(
                            initial,
                            config,
                            stream_mode=["custom", "values"],
                            version="v2",
                        ):
                            if part["type"] == "custom":
                                yield cast(dict[str, Any], part["data"])
                            elif part["type"] == "values":
                                latest = cast(AgentState, part["data"])
                        status = "success"
                        result = _result(latest)
                        yield {"type": "result", "payload": result.model_dump()}
                finally:
                    AGENT_IN_FLIGHT.dec()
                    AGENT_RUNS.labels(status, "true").inc()
                    AGENT_DURATION.labels(status, "true").observe(time.perf_counter() - started_at)
                    AGENT_ITERATIONS.observe(float(latest.get("rewrite_attempts", 0) + 1))

    async def cleanup_completed_checkpoint(self, tenant_id: str, run_id: str) -> None:
        if self._settings.retain_completed_checkpoints or self._checkpointer is None:
            return
        await self._checkpointer.adelete_thread(f"{tenant_id}:{run_id}")

    async def _run(self, request: RunRequest, stream: bool) -> AgentState:
        self._validate_question(request)
        initial = _initial_state(request)
        initial["stream"] = stream
        latest = initial
        async with self._lock_for(request):
            async with self._run_slots:
                AGENT_IN_FLIGHT.inc()
                started_at = time.perf_counter()
                status = "error"
                try:
                    credentials = await self._backend.model_credentials(request)
                    with self._models.use_credentials(credentials):
                        self._models.validate_provider(request.llm_provider)
                        result = await self._graph.ainvoke(initial, _run_config(request))
                        latest = cast(AgentState, result)
                        status = "success"
                        return latest
                finally:
                    AGENT_IN_FLIGHT.dec()
                    AGENT_RUNS.labels(status, str(stream).lower()).inc()
                    AGENT_DURATION.labels(status, str(stream).lower()).observe(
                        time.perf_counter() - started_at
                    )
                    AGENT_ITERATIONS.observe(float(latest.get("rewrite_attempts", 0) + 1))

    def _lock_for(self, request: RunRequest) -> asyncio.Lock:
        key = f"{request.tenant_id}:{request.run_id}"
        lock = self._run_locks.get(key)
        if lock is None:
            lock = asyncio.Lock()
            self._run_locks[key] = lock
        return lock

    def _build_graph(self, checkpointer: Any) -> Any:
        graph = StateGraph(AgentState)
        graph.add_node("route", self._route)
        graph.add_node("retrieve", self._retrieve)
        graph.add_node("overview", self._overview)
        graph.add_node("navigation", self._navigation)
        graph.add_node("web_search", self._web_search)
        graph.add_node("evaluate", self._evaluate)
        graph.add_node("rewrite", self._rewrite)
        graph.add_node("synthesize", self._synthesize)
        graph.add_node("validate", self._validate_answer)
        graph.add_node("refuse", self._refuse)

        graph.add_edge(START, "route")
        graph.add_conditional_edges(
            "route",
            lambda state: state["route"],
            {"rag": "retrieve", "overview": "overview", "navigation": "navigation"},
        )
        graph.add_conditional_edges(
            "retrieve",
            lambda state: "web_search" if state["request"].get("web_search") else "evaluate",
            {"web_search": "web_search", "evaluate": "evaluate"},
        )
        graph.add_edge("web_search", "evaluate")
        graph.add_conditional_edges(
            "overview",
            self._next_after_knowledge_tool,
            {"synthesize": "synthesize", "refuse": "refuse"},
        )
        graph.add_conditional_edges(
            "navigation",
            self._next_after_knowledge_tool,
            {"synthesize": "synthesize", "refuse": "refuse"},
        )
        graph.add_conditional_edges(
            "evaluate",
            self._next_after_evaluation,
            {"rewrite": "rewrite", "synthesize": "synthesize", "refuse": "refuse"},
        )
        graph.add_edge("rewrite", "retrieve")
        graph.add_edge("synthesize", "validate")
        graph.add_edge("validate", END)
        graph.add_edge("refuse", END)
        return graph.compile(checkpointer=checkpointer)

    def _validate_question(self, request: RunRequest) -> None:
        if len(request.question) > self._settings.max_question_characters:
            raise ValueError("question exceeds the configured length limit")

    @traced_node("route")
    async def _route(self, state: AgentState) -> dict[str, Any]:
        request = RunRequest.model_validate(state["request"])
        step = _start_step("planning", "agent.plan", "planning")
        _emit("status", {"message": "agent.plan.start"})
        _emit("agent_step", step.model_dump())
        started_at = time.perf_counter()
        decision = await self._models.route(request.question, request.llm_model)
        step.status = "completed"
        step.state = "executing"
        step.detail = decision.reason
        step.duration_ms = _duration_ms(started_at)
        _emit("agent_step", step.model_dump())
        AGENT_TRANSITIONS.labels("route", "completed").inc()
        return {
            "route": decision.route,
            "search_query": decision.search_query or request.question,
            "topic": decision.topic or request.question,
            "steps": [*state["steps"], step.model_dump()],
        }

    @traced_node("retrieve")
    async def _retrieve(self, state: AgentState) -> dict[str, Any]:
        request = RunRequest.model_validate(state["request"])
        step = _start_step(
            "tool",
            "agent.tool.knowledge_search",
            "executing",
            "knowledge_search",
            state["rewrite_attempts"] + 1,
        )
        _emit("status", {"message": "chat.retrieval.start"})
        _emit("agent_step", step.model_dump())
        started_at = time.perf_counter()
        try:
            with observe_tool("knowledge_search"):
                response = await self._backend.knowledge_search(request, state["search_query"])
        except Exception as exc:
            _fail_step(step, started_at, exc)
            raise
        for stage in response.stages:
            _emit("status", {"message": stage})
        step.status = "completed"
        step.state = "observing"
        step.detail = f"返回{len(response.chunks)}个片段"
        step.duration_ms = _duration_ms(started_at)
        _emit("agent_step", step.model_dump())
        AGENT_TRANSITIONS.labels("retrieve", "completed").inc()
        metrics = response.metrics.model_copy()
        metrics.original_query = request.question
        metrics.search_query = state["search_query"]
        metrics.query_rewritten = state["rewrite_attempts"] > 0
        return {
            "chunks": [chunk.model_dump() for chunk in response.chunks],
            "metrics": metrics.model_dump(),
            "context": response.content,
            "coverage": response.coverage.model_dump(),
            "steps": [*state["steps"], step.model_dump()],
        }

    def _next_after_knowledge_tool(self, state: AgentState) -> str:
        if not state["context"].strip() or not state["chunks"]:
            return "refuse"
        if state["route"] == "overview":
            coverage = ToolCoverage.model_validate(state["coverage"])
            if not coverage.complete:
                return "refuse"
        return "synthesize"

    @traced_node("overview")
    async def _overview(self, state: AgentState) -> dict[str, Any]:
        request = RunRequest.model_validate(state["request"])
        return await self._run_knowledge_tool(
            state,
            "knowledge_overview",
            "agent.tool.knowledge_overview",
            lambda: self._backend.knowledge_overview(request),
        )

    @traced_node("navigation")
    async def _navigation(self, state: AgentState) -> dict[str, Any]:
        request = RunRequest.model_validate(state["request"])
        return await self._run_knowledge_tool(
            state,
            "document_navigation",
            "agent.tool.document_navigation",
            lambda: self._backend.document_navigation(request, state["topic"]),
        )

    async def _run_knowledge_tool(
        self,
        state: AgentState,
        tool: str,
        title: str,
        call: Callable[[], Any],
    ) -> dict[str, Any]:
        step = _start_step("tool", title, "executing", tool, 1)
        status = "chat.route.overview" if tool == "knowledge_overview" else "chat.route.navigation"
        _emit("status", {"message": status})
        _emit("agent_step", step.model_dump())
        started_at = time.perf_counter()
        try:
            with observe_tool(tool):
                response = await call()
        except Exception as exc:
            _fail_step(step, started_at, exc)
            raise
        step.status = "completed"
        step.state = "observing"
        if tool == "knowledge_overview" and response.coverage.total_documents > 0:
            step.detail = (
                f"覆盖{response.coverage.covered_documents}/"
                f"{response.coverage.total_documents}篇文档，"
                f"返回{len(response.chunks)}个代表片段"
            )
        else:
            step.detail = f"返回{len(response.chunks)}个片段"
        step.duration_ms = _duration_ms(started_at)
        _emit("agent_step", step.model_dump())
        AGENT_TRANSITIONS.labels(tool, "completed").inc()
        metrics = response.metrics.model_copy()
        metrics.route = state["route"]
        return {
            "chunks": [chunk.model_dump() for chunk in response.chunks],
            "metrics": metrics.model_dump(),
            "context": response.content,
            "coverage": response.coverage.model_dump(),
            "steps": [*state["steps"], step.model_dump()],
        }

    @traced_node("web_search")
    async def _web_search(self, state: AgentState) -> dict[str, Any]:
        request = RunRequest.model_validate(state["request"])
        step = _start_step("tool", "agent.tool.web_search", "executing", "web_search", 1)
        _emit("status", {"message": "chat.web.searching"})
        _emit("agent_step", step.model_dump())
        started_at = time.perf_counter()
        try:
            with observe_tool("web_search"):
                links = await self._models.search_web(request.question, request.llm_model)
        except Exception as exc:
            _fail_step(step, started_at, exc)
            raise
        step.status = "completed"
        step.state = "observing"
        step.detail = f"返回{len(links)}个网页来源"
        step.duration_ms = _duration_ms(started_at)
        _emit("agent_step", step.model_dump())
        _emit("status", {"message": "chat.web.ready" if links else "chat.web.empty"})
        return {
            "external_links": [link.model_dump() for link in links],
            "steps": [*state["steps"], step.model_dump()],
        }

    @traced_node("evaluate")
    async def _evaluate(self, state: AgentState) -> dict[str, Any]:
        step = _start_step(
            "observation",
            "agent.observe",
            "observing",
            iteration=state["rewrite_attempts"] + 1,
        )
        started_at = time.perf_counter()
        request = RunRequest.model_validate(state["request"])
        source_count = len(state["chunks"]) + len(state["external_links"])
        can_rewrite = (
            state["route"] == "rag"
            and state["rewrite_attempts"] < self._settings.max_rewrite_attempts
        )
        if source_count == 0:
            evaluation = EvidenceEvaluation(
                decision="rewrite" if can_rewrite else "refuse",
                missing_aspects=["未检索到可用资料"],
                reason="no_evidence",
            )
        else:
            chunks = [RetrievalChunk.model_validate(item) for item in state["chunks"]][
                : self._settings.max_citations
            ]
            links = [ExternalLink.model_validate(item) for item in state["external_links"]][
                : max(self._settings.max_citations - len(chunks), 0)
            ]
            context = _format_context(
                state["context"], chunks, links, self._settings.max_context_characters
            )
            evaluation = await self._models.evaluate_evidence(
                request.question,
                context,
                request.llm_model,
                can_rewrite,
            )
            if evaluation.decision == "rewrite" and not can_rewrite:
                evaluation.decision = "refuse"
                evaluation.rewrite_query = ""
        step.status = "completed"
        step.state = {
            "sufficient": "synthesizing",
            "rewrite": "planning",
            "refuse": "completed",
        }[evaluation.decision]
        step.detail = (
            evaluation.reason
            or {
                "sufficient": "资料充分，开始生成回答",
                "rewrite": "资料不足，重新组织检索问题",
                "refuse": "资料不足，明确拒答",
            }[evaluation.decision]
        )
        step.duration_ms = _duration_ms(started_at)
        _emit("agent_step", step.model_dump())
        AGENT_TRANSITIONS.labels("evaluate", evaluation.decision).inc()
        return {
            "evaluation": evaluation.model_dump(),
            "steps": [*state["steps"], step.model_dump()],
        }

    def _next_after_evaluation(self, state: AgentState) -> str:
        evaluation = EvidenceEvaluation.model_validate(state["evaluation"])
        if evaluation.decision == "sufficient":
            return "synthesize"
        if (
            evaluation.decision == "rewrite"
            and state["route"] == "rag"
            and state["rewrite_attempts"] < self._settings.max_rewrite_attempts
        ):
            return "rewrite"
        return "refuse"

    @traced_node("rewrite")
    async def _rewrite(self, state: AgentState) -> dict[str, Any]:
        request = RunRequest.model_validate(state["request"])
        _emit("status", {"message": "retrieval.rewrite.start"})
        evaluation = EvidenceEvaluation.model_validate(state["evaluation"])
        rewritten = evaluation.rewrite_query.strip()
        if not rewritten:
            rewritten = await self._models.rewrite(
                request.question,
                state["search_query"],
                request.llm_model,
            )
        _emit("status", {"message": "retrieval.rewrite.done"})
        metrics = RetrievalMetrics.model_validate(state["metrics"])
        metrics.search_query = rewritten
        metrics.query_rewritten = rewritten != request.question
        return {
            "search_query": rewritten,
            "rewrite_attempts": state["rewrite_attempts"] + 1,
            "metrics": metrics.model_dump(),
        }

    @traced_node("synthesize")
    async def _synthesize(self, state: AgentState) -> dict[str, Any]:
        request = RunRequest.model_validate(state["request"])
        step = _start_step(
            "synthesis",
            "agent.answer",
            "synthesizing",
            iteration=state["rewrite_attempts"] + 1,
        )
        _emit("status", {"message": "agent.answer.start"})
        _emit("agent_step", step.model_dump())
        started_at = time.perf_counter()
        chunks = [RetrievalChunk.model_validate(item) for item in state["chunks"]][
            : self._settings.max_citations
        ]
        remaining_citations = self._settings.max_citations - len(chunks)
        links = [ExternalLink.model_validate(item) for item in state["external_links"]][
            :remaining_citations
        ]
        context = _format_context(
            state["context"], chunks, links, self._settings.max_context_characters
        )

        if not context:
            answer = "无法确定"
        else:
            prompt_template = (
                OVERVIEW_ANSWER_PROMPT if state["route"] == "overview" else ANSWER_PROMPT
            )
            if state["route"] == "overview":
                prompt = prompt_template.format(question=request.question, context=context)
            else:
                evaluation = EvidenceEvaluation.model_validate(state["evaluation"])
                scope = evaluation.reason.strip() or "根据现有资料回答可确认的内容"
                if evaluation.missing_aspects:
                    scope += "；未覆盖内容：" + "、".join(evaluation.missing_aspects)
                prompt = prompt_template.format(
                    question=request.question,
                    evidence_scope=scope,
                    context=context,
                )
            answer = await self._models.answer(
                request.llm_model,
                prompt,
                False,
                lambda _: None,
            )
        step.status = "completed"
        step.state = "validating"
        step.detail = "候选回答生成完成，开始引用校验"
        step.duration_ms = _duration_ms(started_at)
        _emit("agent_step", step.model_dump())
        AGENT_TRANSITIONS.labels("synthesize", "validating").inc()
        return {
            "answer": answer,
            "chunks": [chunk.model_dump() for chunk in chunks],
            "external_links": [link.model_dump() for link in links],
            "steps": [*state["steps"], step.model_dump()],
        }

    @traced_node("validate")
    async def _validate_answer(self, state: AgentState) -> dict[str, Any]:
        request = RunRequest.model_validate(state["request"])
        step = _start_step(
            "validation",
            "agent.citation.validate",
            "validating",
            iteration=state["rewrite_attempts"] + 1,
        )
        _emit("status", {"message": "agent.citation.validate"})
        _emit("agent_step", step.model_dump())
        started_at = time.perf_counter()
        chunks = [RetrievalChunk.model_validate(item) for item in state["chunks"]]
        links = [ExternalLink.model_validate(item) for item in state["external_links"]]
        context = _format_context(
            state["context"], chunks, links, self._settings.max_context_characters
        )
        answer = state["answer"]
        overview = state["route"] == "overview"
        validation = await self._validate_candidate(
            request, answer, context, chunks, links, overview
        )
        if not validation.supported:
            repair_template = OVERVIEW_ANSWER_REPAIR_PROMPT if overview else ANSWER_REPAIR_PROMPT
            repair_prompt = repair_template.format(
                question=request.question,
                answer=answer,
                reason=validation.reason or "引用与结论不匹配",
                context=context,
            )
            answer = await self._models.answer(
                request.llm_model,
                repair_prompt,
                False,
                lambda _: None,
            )
            validation = await self._validate_candidate(
                request, answer, context, chunks, links, overview
            )
        if validation.supported:
            answer, chunks, links = select_referenced_sources(answer, chunks, links)
            step.detail = "引用校验通过"
        else:
            answer, chunks, links = "无法确定", [], []
            step.detail = f"引用校验失败：{validation.reason or 'unsupported_claims'}"
        if state["stream"]:
            _emit("delta", {"content": answer})
        step.status = "completed"
        step.state = "completed"
        step.duration_ms = _duration_ms(started_at)
        _emit("agent_step", step.model_dump())
        AGENT_TRANSITIONS.labels("validate", "passed" if validation.supported else "refused").inc()
        return self._finish_answer(state, request, answer, chunks, links, step)

    async def _validate_candidate(
        self,
        request: RunRequest,
        answer: str,
        context: str,
        chunks: list[RetrievalChunk],
        links: list[ExternalLink],
        overview: bool,
    ) -> CitationValidation:
        structural = validate_reference_structure(answer, chunks, links)
        if not structural.supported or is_no_answer(answer):
            return structural
        return await self._models.validate_answer(
            request.question,
            answer,
            context,
            request.llm_model,
            overview,
        )

    @traced_node("refuse")
    async def _refuse(self, state: AgentState) -> dict[str, Any]:
        request = RunRequest.model_validate(state["request"])
        step = _start_step(
            "refusal",
            "agent.answer.insufficient",
            "completed",
            iteration=state["rewrite_attempts"] + 1,
        )
        step.status = "completed"
        coverage = ToolCoverage.model_validate(state["coverage"])
        if state["route"] == "overview" and coverage.total_documents > coverage.covered_documents:
            step.detail = (
                f"知识概览仅覆盖{coverage.covered_documents}/"
                f"{coverage.total_documents}篇文档，无法可靠回答"
            )
        else:
            step.detail = "资料不足，无法可靠回答"
        _emit("status", {"message": "chat.answer.insufficient"})
        if state["stream"]:
            _emit("delta", {"content": "无法确定"})
        _emit("agent_step", step.model_dump())
        AGENT_TRANSITIONS.labels("refuse", "completed").inc()
        return self._finish_answer(state, request, "无法确定", [], [], step)

    def _finish_answer(
        self,
        state: AgentState,
        request: RunRequest,
        answer: str,
        chunks: list[RetrievalChunk],
        links: list[ExternalLink],
        step: AgentStep,
    ) -> dict[str, Any]:
        metrics = RetrievalMetrics.model_validate(state["metrics"])
        metrics.route = state["route"]
        metrics.route_correct = (
            not request.expected_route or request.expected_route == state["route"]
        )
        metrics.latency_ms = int((time.perf_counter() - state["started_at"]) * 1000)
        metrics.citation_count = len(chunks) + len(links)
        metrics.answered = not is_no_answer(answer)
        if state["route"] == "overview":
            metrics.evaluation_passed = (
                metrics.evaluation_passed
                and ToolCoverage.model_validate(state["coverage"]).complete
            )
        elif state["route"] == "rag":
            metrics.evaluation_passed = (
                metrics.evaluation_passed
                and EvidenceEvaluation.model_validate(state["evaluation"]).decision == "sufficient"
            )
        expected = request.expected_outcome.strip().lower()
        metrics.outcome_correct = (
            metrics.answered
            if expected in {"answer", "answered"}
            else not metrics.answered
            if expected == "no_answer"
            else True
        )
        metrics.evaluation_passed = (
            metrics.evaluation_passed and metrics.route_correct and metrics.outcome_correct
        )

        _emit("sources", {"chunks": [chunk.model_dump() for chunk in chunks]})
        _emit("web_sources", {"links": [link.model_dump() for link in links]})
        _emit("metrics", metrics.model_dump())
        return {
            "answer": answer,
            "chunks": [chunk.model_dump() for chunk in chunks],
            "external_links": [link.model_dump() for link in links],
            "metrics": metrics.model_dump(),
            "steps": [*state["steps"], step.model_dump()],
        }


def _initial_state(request: RunRequest) -> AgentState:
    return AgentState(
        request=request.model_dump(),
        stream=False,
        route="rag",
        search_query=request.question,
        topic=request.question,
        context="",
        coverage=ToolCoverage().model_dump(),
        chunks=[],
        external_links=[],
        metrics=RetrievalMetrics(original_query=request.question).model_dump(),
        answer="",
        evaluation=EvidenceEvaluation(decision="refuse").model_dump(),
        rewrite_attempts=0,
        steps=[],
        error="",
        started_at=time.perf_counter(),
    )


def _run_config(request: RunRequest) -> dict[str, Any]:
    return {
        "configurable": {"thread_id": f"{request.tenant_id}:{request.run_id}"},
        "metadata": {
            "run_id": request.run_id,
            "tenant_id": request.tenant_id,
            "session_id": request.session_id,
            "message_id": request.message_id,
        },
    }


def _result(state: AgentState) -> RunResult:
    return RunResult(
        answer=state["answer"],
        chunks=[RetrievalChunk.model_validate(item) for item in state["chunks"]],
        external_links=[ExternalLink.model_validate(item) for item in state["external_links"]],
        metrics=RetrievalMetrics.model_validate(state["metrics"]),
        agent_steps=[AgentStep.model_validate(item) for item in state["steps"]],
    )


def _format_context(
    tool_content: str,
    chunks: list[RetrievalChunk],
    links: list[ExternalLink],
    limit: int,
) -> str:
    sections: list[str] = []
    if tool_content.strip():
        sections.append(tool_content.strip())
    if chunks:
        sections.append(
            "\n\n".join(
                (
                    f"[引用{index}] 文档：{chunk.doc_name}\n"
                    f"章节：{chunk.section or '无'}\n内容：{chunk.content}"
                )
                for index, chunk in enumerate(chunks, start=1)
            )
        )
    if links:
        sections.append(
            "\n\n".join(
                f"[外链{index}] {link.title}\n网址：{link.url}\n摘要：{link.snippet}"
                for index, link in enumerate(links, start=1)
            )
        )
    return "\n\n".join(sections)[:limit].strip()


def _start_step(
    kind: str,
    title: str,
    state: str,
    tool: str = "",
    iteration: int = 0,
) -> AgentStep:
    return AgentStep(
        id=str(uuid4()),
        kind=kind,
        title=title,
        tool=tool,
        state=state,
        iteration=iteration,
        status="running",
    )


def _fail_step(step: AgentStep, started_at: float, error: Exception) -> None:
    step.status = "failed"
    step.state = "failed"
    step.detail = type(error).__name__
    step.duration_ms = _duration_ms(started_at)
    _emit("agent_step", step.model_dump())


def _duration_ms(started_at: float) -> int:
    return int((time.perf_counter() - started_at) * 1000)


def _writer() -> Callable[[dict[str, Any]], None]:
    try:
        return get_stream_writer()
    except RuntimeError:
        return lambda _: None


def _emit(event_type: str, payload: dict[str, Any]) -> None:
    _writer()({"type": event_type, "payload": payload})
