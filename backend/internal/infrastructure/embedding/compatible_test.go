package embedding

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-rag/backend/internal/infrastructure/observability"
)

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
	vectors, err := NewCompatibleClient("key", "model", server.URL, 2).Embed(ctx, []string{"text"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 1 || got.InputTokens != 5 || got.TotalTokens != 5 || got.CostUSD != 0.0001 {
		t.Fatalf("vectors=%v usage=%+v", vectors, got)
	}
}
