from __future__ import annotations

import time
from collections.abc import Awaitable, Callable, Iterator
from contextlib import contextmanager
from functools import wraps

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.trace import Status, StatusCode
from prometheus_client import Counter, Gauge, Histogram

from enterprise_agent.config import TelemetrySettings

AGENT_RUNS = Counter(
    "enterprise_rag_agent_runs_total",
    "Total LangGraph runs by outcome and response mode.",
    ["status", "stream"],
)
AGENT_DURATION = Histogram(
    "enterprise_rag_agent_run_duration_seconds",
    "LangGraph run latency in seconds.",
    ["status", "stream"],
    buckets=(0.25, 0.5, 1, 2, 5, 10, 20, 45, 90, 180),
)
AGENT_IN_FLIGHT = Gauge("enterprise_rag_agent_runs_in_flight", "Current LangGraph runs.")
AGENT_TRANSITIONS = Counter(
    "enterprise_rag_agent_state_transitions_total",
    "LangGraph node transitions.",
    ["node", "status"],
)
AGENT_ITERATIONS = Histogram(
    "enterprise_rag_agent_iterations",
    "Retrieval attempts used by LangGraph runs.",
    buckets=(1, 2, 3),
)
TOOL_CALLS = Counter(
    "enterprise_rag_agent_tool_calls_total",
    "Agent tool calls by tool and outcome.",
    ["tool", "status"],
)
TOOL_DURATION = Histogram(
    "enterprise_rag_agent_tool_duration_seconds",
    "Agent tool latency in seconds.",
    ["tool", "status"],
    buckets=(0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30),
)
MODEL_CALLS = Counter(
    "enterprise_rag_model_calls_total",
    "Model calls by operation, provider and outcome.",
    ["kind", "operation", "provider", "status"],
)
MODEL_DURATION = Histogram(
    "enterprise_rag_model_call_duration_seconds",
    "Model call latency in seconds.",
    ["kind", "operation", "provider", "status"],
    buckets=(0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 45, 90),
)
MODEL_TOKENS = Counter(
    "enterprise_rag_model_tokens_total",
    "Model tokens by operation, provider and token type.",
    ["kind", "operation", "provider", "token_type"],
)
MODEL_COST = Counter(
    "enterprise_rag_model_cost_usd_total",
    "Model cost reported by the provider in US dollars.",
    ["kind", "operation", "provider"],
)


def configure_telemetry(app: object, settings: TelemetrySettings) -> None:
    provider = TracerProvider(resource=Resource.create({"service.name": settings.service_name}))
    provider.add_span_processor(
        BatchSpanProcessor(OTLPSpanExporter(endpoint=settings.otlp_endpoint, insecure=True))
    )
    trace.set_tracer_provider(provider)
    FastAPIInstrumentor.instrument_app(app)  # type: ignore[arg-type]
    HTTPXClientInstrumentor().instrument()


@contextmanager
def observe_tool(name: str) -> Iterator[None]:
    started_at = time.perf_counter()
    status = "error"
    try:
        yield
        status = "success"
    finally:
        TOOL_CALLS.labels(name, status).inc()
        TOOL_DURATION.labels(name, status).observe(time.perf_counter() - started_at)


def observe_model(
    operation: str,
    status: str,
    duration: float,
    input_tokens: int = 0,
    output_tokens: int = 0,
    cost_usd: float = 0,
    provider: str = "openrouter",
) -> None:
    labels = ("llm", operation, provider)
    MODEL_CALLS.labels(*labels, status).inc()
    MODEL_DURATION.labels(*labels, status).observe(duration)
    total_tokens = max(input_tokens + output_tokens, 0)
    if input_tokens > 0:
        MODEL_TOKENS.labels(*labels, "input").inc(input_tokens)
    if output_tokens > 0:
        MODEL_TOKENS.labels(*labels, "output").inc(output_tokens)
    if total_tokens > 0:
        MODEL_TOKENS.labels(*labels, "total").inc(total_tokens)
    if cost_usd > 0:
        MODEL_COST.labels(*labels).inc(cost_usd)


def traced_node[**P, R](
    name: str,
) -> Callable[[Callable[P, Awaitable[R]]], Callable[P, Awaitable[R]]]:
    def decorator(function: Callable[P, Awaitable[R]]) -> Callable[P, Awaitable[R]]:
        @wraps(function)
        async def wrapped(*args: P.args, **kwargs: P.kwargs) -> R:
            tracer = trace.get_tracer("enterprise_agent.graph")
            with tracer.start_as_current_span(f"agent.{name}") as span:
                span.set_attribute("agent.node", name)
                try:
                    return await function(*args, **kwargs)
                except Exception as exc:
                    span.record_exception(exc)
                    span.set_status(Status(StatusCode.ERROR, type(exc).__name__))
                    raise

        return wrapped

    return decorator
