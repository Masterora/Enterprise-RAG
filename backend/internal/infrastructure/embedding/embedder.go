package embedding

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/config"
)

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

func NewEmbedder(c config.EmbeddingConf) (Embedder, error) {
	switch strings.ToLower(strings.TrimSpace(c.Provider)) {
	case "qwen":
		return NewCompatibleClient(c.ApiKey, c.Model, defaultString(c.BaseURL, "https://dashscope.aliyuncs.com/compatible-mode/v1"), c.Dimension), nil
	case "openrouter":
		return NewCompatibleClient(c.ApiKey, c.Model, defaultString(c.BaseURL, "https://openrouter.ai/api/v1"), c.Dimension), nil
	case "compatible":
		return NewCompatibleClient(c.ApiKey, c.Model, c.BaseURL, c.Dimension), nil
	case "mock":
		return NewMockEmbedder(c.Dimension), nil
	default:
		return nil, errors.New("unsupported embedding provider")
	}
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
