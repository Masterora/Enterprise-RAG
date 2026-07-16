from __future__ import annotations

from typing import Any, Literal, TypedDict

from pydantic import BaseModel, ConfigDict, Field, field_validator


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid")


class RetrievalChunk(StrictModel):
    id: str
    evidence_id: str = ""
    tenant_id: str = ""
    doc_id: str
    doc_name: str
    subject_id: str = ""
    user_id: str = ""
    chunk_index: int = 0
    page: int = 0
    section: str = ""
    content: str
    document_version: int = 1
    content_hash: str = ""
    score: float = 0
    raw_score: float = 0
    source: str = ""


class ExternalLink(StrictModel):
    title: str
    url: str
    snippet: str = ""


class RetrievalMetrics(StrictModel):
    original_query: str = ""
    search_query: str = ""
    query_rewritten: bool = False
    reranked: bool = False
    top_k: int = 0
    candidate_count: int = 0
    returned_count: int = 0
    sub_query_count: int = 0
    expected_count: int = 0
    recall_hit_count: int = 0
    recall_at_k: float = 0
    route: str = ""
    route_correct: bool = True
    latency_ms: int = 0
    evaluation_passed: bool = True
    citation_count: int = 0
    answered: bool = False
    outcome_correct: bool = True


class AgentStep(StrictModel):
    id: str
    kind: str
    title: str
    tool: str = ""
    state: str
    iteration: int = 0
    status: Literal["running", "completed", "failed"]
    detail: str = ""
    duration_ms: int = 0


class RunRequest(StrictModel):
    run_id: str
    tenant_id: str
    session_id: str = ""
    message_id: str = ""
    user_id: str
    subject_id: str
    question: str
    top_k: int = Field(default=5, ge=1, le=20)
    llm_provider: str = ""
    llm_model: str = ""
    web_search: bool = False
    expected_doc_ids: list[str] = Field(default_factory=list)
    expected_chunk_ids: list[str] = Field(default_factory=list)
    expected_route: str = ""
    expected_outcome: str = ""

    @field_validator("question")
    @classmethod
    def normalize_question(cls, value: str) -> str:
        normalized = " ".join(value.split())
        if not normalized:
            raise ValueError("question is required")
        return normalized


class RunResult(StrictModel):
    answer: str
    chunks: list[RetrievalChunk] = Field(default_factory=list)
    external_links: list[ExternalLink] = Field(default_factory=list)
    metrics: RetrievalMetrics = Field(default_factory=RetrievalMetrics)
    agent_steps: list[AgentStep] = Field(default_factory=list)


class CheckpointCleanupRequest(StrictModel):
    tenant_id: str = Field(min_length=1)
    run_id: str = Field(min_length=1)


class ToolCoverage(StrictModel):
    total_documents: int = Field(default=0, ge=0)
    covered_documents: int = Field(default=0, ge=0)
    complete: bool = False


class ToolResponse(StrictModel):
    content: str = ""
    chunks: list[RetrievalChunk] = Field(default_factory=list)
    metrics: RetrievalMetrics = Field(default_factory=RetrievalMetrics)
    stages: list[str] = Field(default_factory=list)
    coverage: ToolCoverage = Field(default_factory=ToolCoverage)


class ModelCredentials(StrictModel):
    provider: Literal["openrouter", "openai_compatible"]
    base_url: str = Field(min_length=1)
    api_key: str = Field(min_length=1)


class StreamEvent(StrictModel):
    sequence: int
    type: Literal[
        "status", "agent_step", "sources", "web_sources", "metrics", "delta", "result", "error"
    ]
    payload: dict[str, Any]


RouteName = Literal["rag", "overview", "navigation"]


class RouteDecision(StrictModel):
    route: RouteName = "rag"
    search_query: str = ""
    topic: str = ""
    reason: str = ""


class EvidenceEvaluation(StrictModel):
    decision: Literal["sufficient", "rewrite", "refuse"]
    coverage_score: float = Field(default=0, ge=0, le=1)
    authority_score: float = Field(default=0, ge=0, le=1)
    conflict_detected: bool = False
    missing_aspects: list[str] = Field(default_factory=list)
    rewrite_query: str = ""
    reason: str = ""


class CitationValidation(StrictModel):
    supported: bool
    invalid_references: list[str] = Field(default_factory=list)
    unsupported_claims: list[str] = Field(default_factory=list)
    reason: str = ""


class AgentState(TypedDict):
    request: dict[str, Any]
    stream: bool
    route: str
    search_query: str
    topic: str
    context: str
    coverage: dict[str, Any]
    chunks: list[dict[str, Any]]
    external_links: list[dict[str, Any]]
    metrics: dict[str, Any]
    answer: str
    evaluation: dict[str, Any]
    rewrite_attempts: int
    steps: list[dict[str, Any]]
    error: str
    started_at: float
