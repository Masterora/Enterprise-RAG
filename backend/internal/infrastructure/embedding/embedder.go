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
	case "openai":
		return NewOpenAIClient(c.ApiKey, c.Model), nil
	case "mock":
		return NewMockEmbedder(c.Dimension), nil
	default:
		return nil, errors.New("unsupported embedding provider")
	}
}
