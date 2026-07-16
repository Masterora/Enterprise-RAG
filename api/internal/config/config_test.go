package config

import (
	"path/filepath"
	"testing"
)

func TestLoadMergesIncludedYAMLAndExpandsEnvironment(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "secret")
	t.Setenv("AGENT_SERVICE_TOKEN", "test-agent-service-token")
	t.Setenv("RAG_AUTH_SECRET", "test-auth-secret-0123456789abcdef")
	t.Setenv("REDIS_PASSWORD", "test-redis-password")

	loaded, err := Load(filepath.Join("..", "..", "etc", "rag-api.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "rag-api" || loaded.Embedding.Model != "openai/text-embedding-3-small" || loaded.Embedding.ApiKey != "secret" {
		t.Fatalf("unexpected config: name=%q model=%q key=%q", loaded.Name, loaded.Embedding.Model, loaded.Embedding.ApiKey)
	}
	if loaded.AgentService.URL != "http://localhost:8000" || loaded.AgentService.ServiceToken != "test-agent-service-token" {
		t.Fatalf("unexpected agent service config: %+v", loaded.AgentService)
	}
	if loaded.AgentService.TimeoutSeconds != 90 || !loaded.Metrics.Enabled || loaded.Metrics.Path != "/metrics" {
		t.Fatalf("unexpected runtime config: agent_service=%+v metrics=%+v", loaded.AgentService, loaded.Metrics)
	}
	if loaded.Telemetry.Endpoint != "localhost:4317" || loaded.Telemetry.Batcher != "otlpgrpc" || loaded.Telemetry.Sampler != 1 {
		t.Fatalf("unexpected telemetry config: %+v", loaded.Telemetry)
	}
}

func TestResolveIncludePathRejectsParentDirectory(t *testing.T) {
	if _, err := resolveIncludePath(t.TempDir(), "../secret.yaml"); err == nil {
		t.Fatal("expected parent directory include to fail")
	}
}

func TestLoadAppliesContainerRuntimeOverrides(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "secret")
	t.Setenv("AGENT_SERVICE_TOKEN", "test-agent-service-token")
	t.Setenv("RAG_AUTH_SECRET", "test-auth-secret-0123456789abcdef")
	t.Setenv("REDIS_PASSWORD", "test-redis-password")
	t.Setenv("RAG_POSTGRES_DSN", "postgres://rag:rag@postgres:5432/rag")
	t.Setenv("RAG_AGENT_URL", "http://agent:8000/")
	t.Setenv("RAG_MILVUS_ADDRESS", "milvus:19530")
	t.Setenv("RAG_EMBEDDING_BASE_URL", "http://model:8080/v1")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")

	loaded, err := Load(filepath.Join("..", "..", "etc", "rag-api.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Postgres.DataSource != "postgres://rag:rag@postgres:5432/rag" ||
		loaded.AgentService.URL != "http://agent:8000" ||
		loaded.Milvus.Address != "milvus:19530" ||
		loaded.Embedding.BaseURL != "http://model:8080/v1" ||
		loaded.Telemetry.Endpoint != "otel-collector:4317" {
		t.Fatalf("runtime overrides were not applied: %+v", loaded)
	}
}

func TestLoadRejectsMissingOrWeakAuthSecret(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "secret")
	t.Setenv("AGENT_SERVICE_TOKEN", "test-agent-service-token")
	t.Setenv("RAG_AUTH_SECRET", "short")
	t.Setenv("REDIS_PASSWORD", "test-redis-password")

	if _, err := Load(filepath.Join("..", "..", "etc", "rag-api.yaml")); err == nil {
		t.Fatal("expected weak auth secret to fail")
	}
}

func TestLoadAppliesRateLimitOverride(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "secret")
	t.Setenv("AGENT_SERVICE_TOKEN", "test-agent-service-token")
	t.Setenv("RAG_AUTH_SECRET", "test-auth-secret-0123456789abcdef")
	t.Setenv("REDIS_PASSWORD", "test-redis-password")
	t.Setenv("RAG_RATE_LIMIT_CHAT_QUOTA", "2")

	loaded, err := Load(filepath.Join("..", "..", "etc", "rag-api.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RateLimit.ChatQuota != 2 {
		t.Fatalf("chat quota = %d, want 2", loaded.RateLimit.ChatQuota)
	}
}
