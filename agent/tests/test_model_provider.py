from __future__ import annotations

import pytest

from enterprise_agent.config import ModelSettings
from enterprise_agent.model_provider import MAX_MODEL_CLIENTS, ModelProvider
from enterprise_agent.models import ModelCredentials


@pytest.mark.asyncio
async def test_model_clients_are_cached_and_share_http_pool() -> None:
    provider = ModelProvider(
        ModelSettings(
            default_model="openai/test-model",
            api_key="test-key",
            base_url="https://example.com/v1",
        )
    )
    try:
        first = provider._model("")
        second = provider._model("openai/test-model")

        assert first is second
        assert first.http_async_client is provider._http
    finally:
        await provider.close()


@pytest.mark.asyncio
async def test_model_client_cache_is_bounded() -> None:
    provider = ModelProvider(
        ModelSettings(
            default_model="openai/test-model",
            api_key="test-key",
            base_url="https://example.com/v1",
        )
    )
    try:
        first = provider._model("openai/model-0")
        for index in range(1, MAX_MODEL_CLIENTS + 1):
            provider._model(f"openai/model-{index}")

        assert len(provider._models) == MAX_MODEL_CLIENTS
        assert all(key[-1] != "openai/model-0" for key in provider._models)
        assert provider._model("openai/model-0") is not first
    finally:
        await provider.close()


@pytest.mark.asyncio
async def test_tenant_credentials_are_scoped_and_key_rotation_rebuilds_client() -> None:
    provider = ModelProvider(
        ModelSettings(
            default_model="openai/test-model",
            api_key="fallback-key",
            base_url="https://fallback.example/v1",
        )
    )
    first_credentials = ModelCredentials(
        provider="openrouter",
        api_key="tenant-key-1",
        base_url="https://tenant.example/v1",
    )
    rotated_credentials = first_credentials.model_copy(update={"api_key": "tenant-key-2"})
    try:
        with provider.use_credentials(first_credentials):
            first = provider._model("")
            assert first.openai_api_base == "https://tenant.example/v1"
        with provider.use_credentials(rotated_credentials):
            rotated = provider._model("")
        assert rotated is not first
        assert provider._active_credentials().api_key == "fallback-key"
    finally:
        await provider.close()


@pytest.mark.asyncio
async def test_openai_compatible_provider_is_supported() -> None:
    settings = ModelSettings(
        provider="openai_compatible",
        default_model="model",
        api_key="test-key",
        base_url="https://example.com/v1",
    )
    provider = ModelProvider(settings)
    try:
        provider.validate_provider("openai_compatible")
        with pytest.raises(ValueError, match="not configured"):
            provider.validate_provider("openrouter")
    finally:
        await provider.close()
