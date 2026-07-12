package llm

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/types"
)

type Client interface {
	Generate(ctx context.Context, prompt string, webSearch bool) (string, error)
	GenerateStream(ctx context.Context, prompt string, webSearch bool, onDelta func(string) error) error
	SearchWeb(ctx context.Context, query string) ([]types.ExternalLink, error)
}

func NewClient(c config.ProviderConf) (Client, error) {
	switch strings.ToLower(strings.TrimSpace(c.Provider)) {
	case "qwen":
		return NewCompatibleClient(c.ApiKey, c.Model, defaultString(c.BaseURL, "https://dashscope.aliyuncs.com/compatible-mode/v1")), nil
	case "openrouter":
		return NewCompatibleClient(c.ApiKey, c.Model, defaultString(c.BaseURL, "https://openrouter.ai/api/v1")), nil
	case "compatible":
		return NewCompatibleClient(c.ApiKey, c.Model, c.BaseURL), nil
	default:
		return nil, errors.New("unsupported llm provider")
	}
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
