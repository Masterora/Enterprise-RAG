package llm

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

func (c *OpenAIClient) Generate(ctx context.Context, prompt string) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("OPENAI_API_KEY is required")
	}
	if strings.TrimSpace(c.model) == "" {
		return "", errors.New("openai model is required")
	}

	body, err := json.Marshal(map[string]any{
		"model":             c.model,
		"input":             prompt,
		"max_output_tokens": 800,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai responses request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	var parsed struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", err
	}
	if strings.TrimSpace(parsed.OutputText) != "" {
		return strings.TrimSpace(parsed.OutputText), nil
	}

	var builder strings.Builder
	for _, item := range parsed.Output {
		for _, content := range item.Content {
			builder.WriteString(content.Text)
		}
	}
	answer := strings.TrimSpace(builder.String())
	if answer == "" {
		return "", errors.New("openai response text is empty")
	}
	return answer, nil
}

func (c *OpenAIClient) GenerateStream(ctx context.Context, prompt string, onDelta func(string) error) error {
	answer, err := c.Generate(ctx, prompt)
	if err != nil {
		return err
	}
	return onDelta(answer)
}
