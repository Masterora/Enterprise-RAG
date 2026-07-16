from __future__ import annotations

import os
from pathlib import Path
from typing import Any, Literal, cast

import yaml
from pydantic import BaseModel, ConfigDict, Field, model_validator


class ServiceSettings(BaseModel):
    model_config = ConfigDict(extra="forbid")

    host: str = "0.0.0.0"
    port: int = Field(default=8000, ge=1, le=65535)
    service_token: str = Field(min_length=16)
    request_timeout_seconds: int = Field(default=90, ge=1, le=300)


class BackendSettings(BaseModel):
    model_config = ConfigDict(extra="forbid")

    base_url: str
    timeout_seconds: int = Field(default=30, ge=1, le=120)


class AgentSettings(BaseModel):
    model_config = ConfigDict(extra="forbid")

    max_rewrite_attempts: int = Field(default=1, ge=0, le=2)
    max_question_characters: int = Field(default=4000, ge=100, le=20000)
    max_context_characters: int = Field(default=18000, ge=1000, le=50000)
    max_citations: int = Field(default=5, ge=1, le=10)
    max_concurrent_runs: int = Field(default=32, ge=1, le=256)
    retain_completed_checkpoints: bool = False


class ModelSettings(BaseModel):
    model_config = ConfigDict(extra="forbid")

    provider: Literal["openrouter", "openai_compatible"] = "openrouter"
    default_model: str
    api_key: str = Field(min_length=1)
    base_url: str
    timeout_seconds: int = Field(default=45, ge=1, le=120)
    max_retries: int = Field(default=2, ge=0, le=4)
    max_output_tokens: int = Field(default=2000, ge=128, le=16000)

    @model_validator(mode="after")
    def validate_provider(self) -> ModelSettings:
        provider = self.provider.strip().lower()
        if provider not in {"openrouter", "openai_compatible"}:
            raise ValueError("model provider must be openrouter or openai_compatible")
        self.provider = cast(Literal["openrouter", "openai_compatible"], provider)
        return self


class CheckpointSettings(BaseModel):
    model_config = ConfigDict(extra="forbid")

    postgres_dsn: str
    schema_name: str = Field(default="langgraph", pattern=r"^[a-z][a-z0-9_]{0,62}$")
    setup_on_start: bool = True


class TelemetrySettings(BaseModel):
    model_config = ConfigDict(extra="forbid")

    service_name: str = "enterprise-rag-agent"
    otlp_endpoint: str = "http://localhost:4317"
    metrics_enabled: bool = True


class Settings(BaseModel):
    model_config = ConfigDict(extra="forbid")

    service: ServiceSettings
    backend: BackendSettings
    agent: AgentSettings
    model: ModelSettings
    checkpoint: CheckpointSettings
    telemetry: TelemetrySettings


def load_settings(path: str | Path | None = None) -> Settings:
    config_value: str | Path = (
        path if path is not None else os.getenv("AGENT_CONFIG", "config/application.yaml")
    )
    config_path = Path(config_value)
    raw = yaml.safe_load(config_path.read_text(encoding="utf-8"))
    expanded = _expand_environment(raw)
    _apply_runtime_overrides(expanded)
    return Settings.model_validate(expanded)


def _expand_environment(value: Any) -> Any:
    if isinstance(value, dict):
        return {key: _expand_environment(item) for key, item in value.items()}
    if isinstance(value, list):
        return [_expand_environment(item) for item in value]
    if isinstance(value, str):
        return os.path.expandvars(value)
    return value


def _apply_runtime_overrides(config: Any) -> None:
    if not isinstance(config, dict):
        return
    overrides = {
        ("backend", "base_url"): "AGENT_BACKEND_URL",
        ("model", "base_url"): "AGENT_MODEL_BASE_URL",
        ("checkpoint", "postgres_dsn"): "AGENT_POSTGRES_DSN",
        ("telemetry", "otlp_endpoint"): "OTEL_EXPORTER_OTLP_ENDPOINT",
    }
    for (section, key), environment_name in overrides.items():
        value = os.getenv(environment_name)
        target = config.get(section)
        if value and isinstance(target, dict):
            target[key] = value
