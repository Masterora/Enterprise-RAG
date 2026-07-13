package config

import (
	"path/filepath"
	"testing"
)

func TestLoadMergesIncludedYAMLAndExpandsEnvironment(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "secret")

	loaded, err := Load(filepath.Join("..", "..", "etc", "rag-api.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "rag-api" || loaded.LLM.Model != "openai/gpt-5.6-sol" || loaded.LLM.ApiKey != "secret" || !loaded.Agent.Enabled {
		t.Fatalf("unexpected config: name=%q model=%q key=%q agent_enabled=%t", loaded.Name, loaded.LLM.Model, loaded.LLM.ApiKey, loaded.Agent.Enabled)
	}
	if len(loaded.Agent.EnabledTools) != 4 {
		t.Fatalf("enabled agent tools = %d, want 4", len(loaded.Agent.EnabledTools))
	}
	if loaded.Agent.MaxIterations != 3 || loaded.Agent.MaxTotalTools != 6 || !loaded.Metrics.Enabled || loaded.Metrics.Path != "/metrics" {
		t.Fatalf("unexpected runtime config: agent=%+v metrics=%+v", loaded.Agent, loaded.Metrics)
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
