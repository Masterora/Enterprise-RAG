package llm

import (
	"bufio"
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

type OpenAICompatibleClient struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewOpenAICompatibleClient(apiKey, model, baseURL string) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *OpenAICompatibleClient) Generate(ctx context.Context, prompt string) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("llm api key is required")
	}
	if strings.TrimSpace(c.model) == "" {
		return "", errors.New("llm model is required")
	}
	if c.baseURL == "" {
		return "", errors.New("llm base url is required")
	}

	body, err := json.Marshal(map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":      800,
		"enable_thinking": false,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
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
		return "", fmt.Errorf("llm compatible chat request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("llm response choices is empty")
	}

	answer := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if answer == "" {
		return "", errors.New("llm response text is empty")
	}
	return answer, nil
}

func (c *OpenAICompatibleClient) GenerateStream(ctx context.Context, prompt string, onDelta func(string) error) error {
	if c.apiKey == "" {
		return errors.New("llm api key is required")
	}
	if strings.TrimSpace(c.model) == "" {
		return errors.New("llm model is required")
	}
	if c.baseURL == "" {
		return errors.New("llm base url is required")
	}

	body, err := json.Marshal(map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":      800,
		"stream":          true,
		"enable_thinking": false,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return readErr
		}
		return fmt.Errorf("llm compatible stream request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}

		var parsed struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			return err
		}
		shouldStop := false
		for _, choice := range parsed.Choices {
			if choice.Delta.Content == "" {
				if choice.FinishReason != "" {
					shouldStop = true
				}
				continue
			}
			if err := onDelta(choice.Delta.Content); err != nil {
				return err
			}
			if choice.FinishReason != "" {
				shouldStop = true
			}
		}
		if shouldStop {
			return nil
		}
	}
	return scanner.Err()
}
