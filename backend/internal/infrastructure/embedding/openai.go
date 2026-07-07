package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAIClient struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenAIClient(apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *OpenAIClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, errors.New("embedding api key is required")
	}
	if strings.TrimSpace(c.model) == "" {
		return nil, errors.New("embedding model is required")
	}

	body, err := json.Marshal(map[string]any{
		"model": c.model,
		"input": texts,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai embeddings request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embedding response count mismatch: want=%d got=%d", len(texts), len(parsed.Data))
	}

	vectors := make([][]float32, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		if len(item.Embedding) == 0 {
			return nil, errors.New("embedding vector is empty")
		}
		vectors = append(vectors, item.Embedding)
	}
	return vectors, nil
}
