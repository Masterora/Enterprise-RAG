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

const compatibleEmbeddingBatchSize = 10

type CompatibleClient struct {
	apiKey    string
	model     string
	baseURL   string
	dimension int
	client    *http.Client
}

func NewCompatibleClient(apiKey, model, baseURL string, dimension int) *CompatibleClient {
	return &CompatibleClient{
		apiKey:    apiKey,
		model:     model,
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		dimension: dimension,
		client:    &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *CompatibleClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, errors.New("embedding api key is required")
	}
	if strings.TrimSpace(c.model) == "" {
		return nil, errors.New("embedding model is required")
	}
	if c.baseURL == "" {
		return nil, errors.New("embedding base url is required")
	}

	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += compatibleEmbeddingBatchSize {
		end := start + compatibleEmbeddingBatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batchVectors, err := c.embedBatch(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, batchVectors...)
	}
	return vectors, nil
}

func (c *CompatibleClient) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	payload := map[string]any{
		"model": c.model,
		"input": texts,
	}
	if c.dimension > 0 {
		payload["dimensions"] = c.dimension
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
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
		return nil, fmt.Errorf("embedding compatible request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
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
