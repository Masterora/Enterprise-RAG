package embedding

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"enterprise-rag/api/internal/config"
)

type Embedder interface {
	Embed(ctx context.Context, tenantID string, texts []string) ([][]float32, error)
	ValidateAPIKey(ctx context.Context, apiKey string) error
	Close()
}

type APIKeyResolver interface {
	ResolveAPIKey(ctx context.Context, tenantID string) (string, error)
}

type resolvingEmbedder struct {
	config   config.EmbeddingConf
	resolver APIKeyResolver
	client   *http.Client
}

type staticAPIKeyResolver struct {
	apiKey string
}

func (r staticAPIKeyResolver) ResolveAPIKey(context.Context, string) (string, error) {
	return r.apiKey, nil
}

func ProviderName(c config.EmbeddingConf) string {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" {
		return "openrouter"
	}
	return provider
}

func NewEmbedder(c config.EmbeddingConf, resolvers ...APIKeyResolver) (Embedder, error) {
	provider := ProviderName(c)
	if provider != "openrouter" && provider != "openai_compatible" {
		return nil, errors.New("embedding provider must be openrouter or openai_compatible")
	}
	apiKey := strings.TrimSpace(c.ApiKey)
	model := strings.TrimSpace(c.Model)
	baseURL := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if model == "" {
		return nil, errors.New("embedding model is required")
	}
	if baseURL == "" {
		return nil, errors.New("embedding base url is required")
	}
	if c.Dimension < 1 {
		return nil, errors.New("embedding dimension must be positive")
	}
	var resolver APIKeyResolver
	if len(resolvers) > 0 && resolvers[0] != nil {
		resolver = resolvers[0]
	} else {
		if apiKey == "" {
			return nil, errors.New("embedding api key is required")
		}
		resolver = staticAPIKeyResolver{apiKey: apiKey}
	}
	c.Model = model
	c.BaseURL = baseURL
	return &resolvingEmbedder{
		config:   c,
		resolver: resolver,
		client:   &http.Client{Timeout: 90 * time.Second},
	}, nil
}

func (e *resolvingEmbedder) Embed(ctx context.Context, tenantID string, texts []string) ([][]float32, error) {
	apiKey, err := e.resolver.ResolveAPIKey(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	client := newOpenRouterClient(apiKey, e.config.Model, e.config.BaseURL, e.config.Dimension, e.client, false)
	return client.Embed(ctx, texts)
}

func (e *resolvingEmbedder) ValidateAPIKey(ctx context.Context, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return errors.New("API key is required")
	}
	client := newOpenRouterClient(apiKey, e.config.Model, e.config.BaseURL, e.config.Dimension, e.client, false)
	vectors, err := client.Embed(ctx, []string{"Enterprise-RAG configuration check"})
	if err != nil {
		return err
	}
	if len(vectors) != 1 || len(vectors[0]) != e.config.Dimension {
		return errors.New("embedding model returned an unexpected vector dimension")
	}
	return nil
}

func (e *resolvingEmbedder) Close() {
	e.client.CloseIdleConnections()
}
