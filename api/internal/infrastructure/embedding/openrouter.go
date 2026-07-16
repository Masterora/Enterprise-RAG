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

	"enterprise-rag/api/internal/infrastructure/observability"
)

const openRouterEmbeddingBatchSize = 10

type OpenRouterClient struct {
	apiKey     string
	model      string
	baseURL    string
	dimension  int
	client     *http.Client
	ownsClient bool
}

func NewOpenRouterClient(apiKey, model, baseURL string, dimension int) *OpenRouterClient {
	return newOpenRouterClient(
		apiKey,
		model,
		baseURL,
		dimension,
		&http.Client{Timeout: 90 * time.Second},
		true,
	)
}

func newOpenRouterClient(apiKey, model, baseURL string, dimension int, client *http.Client, ownsClient bool) *OpenRouterClient {
	return &OpenRouterClient{
		apiKey:     apiKey,
		model:      model,
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		dimension:  dimension,
		client:     client,
		ownsClient: ownsClient,
	}
}

func (c *OpenRouterClient) Close() {
	if c.ownsClient {
		c.client.CloseIdleConnections()
	}
}

func (c *OpenRouterClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += openRouterEmbeddingBatchSize {
		end := start + openRouterEmbeddingBatchSize
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

func (c *OpenRouterClient) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
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
		return nil, fmt.Errorf("openrouter embedding request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			PromptTokens int     `json:"prompt_tokens"`
			TotalTokens  int     `json:"total_tokens"`
			Cost         float64 `json:"cost"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embedding response count mismatch: want=%d got=%d", len(texts), len(parsed.Data))
	}
	observability.ReportModelUsage(ctx, observability.ModelUsage{
		InputTokens: parsed.Usage.PromptTokens,
		TotalTokens: parsed.Usage.TotalTokens,
		CostUSD:     parsed.Usage.Cost,
	})

	vectors := make([][]float32, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		if len(item.Embedding) == 0 {
			return nil, errors.New("embedding vector is empty")
		}
		vectors = append(vectors, item.Embedding)
	}
	return vectors, nil
}
