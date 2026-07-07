package llm

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/config"
)

type Client interface {
	Generate(ctx context.Context, prompt string) (string, error)
	GenerateStream(ctx context.Context, prompt string, onDelta func(string) error) error
}

func NewClient(c config.ProviderConf) (Client, error) {
	switch strings.ToLower(strings.TrimSpace(c.Provider)) {
	case "openai":
		return NewOpenAIClient(c.ApiKey, c.Model), nil
	case "qwen":
		return NewOpenAICompatibleClient(c.ApiKey, c.Model, defaultString(c.BaseURL, "https://dashscope.aliyuncs.com/compatible-mode/v1")), nil
	case "openrouter":
		return NewOpenAICompatibleClient(c.ApiKey, c.Model, defaultString(c.BaseURL, "https://openrouter.ai/api/v1")), nil
	case "openai_compatible":
		return NewOpenAICompatibleClient(c.ApiKey, c.Model, c.BaseURL), nil
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
