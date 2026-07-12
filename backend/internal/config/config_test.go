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
	if loaded.Name != "rag-api" || loaded.LLM.Model != "openai/gpt-5.6-sol" || loaded.LLM.ApiKey != "secret" {
		t.Fatalf("unexpected config: name=%q model=%q key=%q", loaded.Name, loaded.LLM.Model, loaded.LLM.ApiKey)
	}
	cases, err := LoadEvaluationCases(loaded.Evaluation.CasesFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 6 {
		t.Fatalf("evaluation case count = %d, want at least 6", len(cases))
	}
}

func TestResolveIncludePathRejectsParentDirectory(t *testing.T) {
	if _, err := resolveIncludePath(t.TempDir(), "../secret.yaml"); err == nil {
		t.Fatal("expected parent directory include to fail")
	}
}
