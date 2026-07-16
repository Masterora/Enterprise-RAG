from __future__ import annotations

from pathlib import Path

import pytest

from enterprise_agent.config import load_settings


def test_load_settings_applies_container_runtime_overrides(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = tmp_path / "application.yaml"
    config.write_text(
        """
service:
  service_token: ${AGENT_SERVICE_TOKEN}
backend:
  base_url: http://localhost:9999
agent: {}
model:
  default_model: e2e-model
  api_key: ${OPENROUTER_API_KEY}
  base_url: https://openrouter.ai/api/v1
checkpoint:
  postgres_dsn: postgres://localhost/rag
telemetry: {}
""".strip(),
        encoding="utf-8",
    )
    monkeypatch.setenv("AGENT_SERVICE_TOKEN", "0123456789abcdef")
    monkeypatch.setenv("OPENROUTER_API_KEY", "secret")
    monkeypatch.setenv("AGENT_BACKEND_URL", "http://api:9999")
    monkeypatch.setenv("AGENT_MODEL_BASE_URL", "http://model:8080/v1")
    monkeypatch.setenv("AGENT_POSTGRES_DSN", "postgres://postgres/rag")
    monkeypatch.setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4317")

    settings = load_settings(config)

    assert settings.backend.base_url == "http://api:9999"
    assert settings.model.base_url == "http://model:8080/v1"
    assert settings.checkpoint.postgres_dsn == "postgres://postgres/rag"
    assert settings.telemetry.otlp_endpoint == "http://otel-collector:4317"
