package embedding

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/infrastructure/observability"
)

type testAPIKeyResolver struct {
	keys map[string]string
}

func (r testAPIKeyResolver) ResolveAPIKey(_ context.Context, tenantID string) (string, error) {
	return r.keys[tenantID], nil
}

func TestEmbedReportsProviderUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[{"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":5,"total_tokens":5,"cost":0.0001}}`)
	}))
	defer server.Close()

	var got observability.ModelUsage
	ctx := observability.WithModelUsageObserver(context.Background(), func(usage observability.ModelUsage) {
		got = usage
	})
	vectors, err := NewOpenRouterClient("key", "model", server.URL, 2).Embed(ctx, []string{"text"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 1 || got.InputTokens != 5 || got.TotalTokens != 5 || got.CostUSD != 0.0001 {
		t.Fatalf("vectors=%v usage=%+v", vectors, got)
	}
}

func TestNewEmbedderRejectsIncompleteConfiguration(t *testing.T) {
	if _, err := NewEmbedder(config.EmbeddingConf{}); err == nil {
		t.Fatal("expected incomplete OpenRouter configuration to fail")
	}
}

func TestResolvingEmbedderUsesTenantAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tenant-a-key" {
			http.Error(w, "unexpected authorization", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[{"embedding":[0.1,0.2]}]}`)
	}))
	defer server.Close()

	embedder, err := NewEmbedder(
		config.EmbeddingConf{Provider: "openrouter", Model: "model", BaseURL: server.URL, Dimension: 2},
		testAPIKeyResolver{keys: map[string]string{"tenant-a": "tenant-a-key"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer embedder.Close()
	if _, err := embedder.Embed(context.Background(), "tenant-a", []string{"text"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateAPIKeyRejectsUnexpectedDimension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[{"embedding":[0.1]}]}`)
	}))
	defer server.Close()

	embedder, err := NewEmbedder(
		config.EmbeddingConf{Provider: "openrouter", Model: "model", BaseURL: server.URL, Dimension: 2},
		testAPIKeyResolver{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer embedder.Close()
	if err := embedder.ValidateAPIKey(context.Background(), "key"); err == nil {
		t.Fatal("expected dimension validation error")
	}
}
