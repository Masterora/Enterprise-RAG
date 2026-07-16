package agent

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

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/types"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Request struct {
	RunID            string   `json:"run_id"`
	TenantID         string   `json:"tenant_id"`
	SessionID        string   `json:"session_id"`
	MessageID        string   `json:"message_id"`
	UserID           string   `json:"user_id"`
	SubjectID        string   `json:"subject_id"`
	Question         string   `json:"question"`
	TopK             int      `json:"top_k"`
	LLMProvider      string   `json:"llm_provider"`
	LLMModel         string   `json:"llm_model"`
	WebSearch        bool     `json:"web_search"`
	ExpectedDocIDs   []string `json:"expected_doc_ids"`
	ExpectedChunkIDs []string `json:"expected_chunk_ids"`
	ExpectedRoute    string   `json:"expected_route"`
	ExpectedOutcome  string   `json:"expected_outcome"`
}

type Result struct {
	Answer        string                 `json:"answer"`
	Chunks        []types.RetrievalChunk `json:"chunks"`
	ExternalLinks []types.ExternalLink   `json:"external_links"`
	Metrics       types.RetrievalMetrics `json:"metrics"`
	AgentSteps    []types.AgentStep      `json:"agent_steps"`
}

type Callbacks struct {
	OnStatus     func(string) error
	OnAgentStep  func(types.AgentStep) error
	OnSources    func([]types.RetrievalChunk) error
	OnWebSources func([]types.ExternalLink) error
	OnMetrics    func(types.RetrievalMetrics) error
	OnDelta      func(string) error
}

type Client struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

func NewClient(c config.AgentServiceConf) *Client {
	return &Client{
		baseURL:      strings.TrimRight(c.URL, "/"),
		serviceToken: c.ServiceToken,
		httpClient:   &http.Client{Timeout: time.Duration(c.TimeoutSeconds) * time.Second},
	}
}

func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

func (c *Client) Ready(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ready", nil)
	if err != nil {
		return fmt.Errorf("create agent readiness request: %w", err)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("check agent readiness: %w", err)
	}
	defer response.Body.Close()
	if err := responseError(response); err != nil {
		return fmt.Errorf("check agent readiness: %w", err)
	}
	return nil
}

func (c *Client) Invoke(ctx context.Context, request Request) (Result, error) {
	response, err := c.post(ctx, "/internal/v1/runs/invoke", request, "application/json")
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	if err := responseError(response); err != nil {
		return Result{}, err
	}
	var result Result
	if err := decodeJSON(io.LimitReader(response.Body, 8<<20), &result); err != nil {
		return Result{}, fmt.Errorf("decode agent response: %w", err)
	}
	return result, nil
}

func (c *Client) Cleanup(ctx context.Context, tenantID, runID string) error {
	response, err := c.post(ctx, "/internal/v1/runs/cleanup", map[string]string{
		"tenant_id": tenantID,
		"run_id":    runID,
	}, "application/json")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return responseError(response)
}

func (c *Client) Stream(ctx context.Context, request Request, callbacks Callbacks) (Result, error) {
	response, err := c.post(ctx, "/internal/v1/runs/stream", request, "application/x-ndjson")
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	if err := responseError(response); err != nil {
		return Result{}, err
	}

	var result Result
	foundResult := false
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 8<<20)
	for scanner.Scan() {
		var event streamEvent
		if err := decodeJSON(bytes.NewReader(scanner.Bytes()), &event); err != nil {
			return Result{}, fmt.Errorf("decode agent stream event: %w", err)
		}
		if err := dispatchEvent(event, callbacks, &result, &foundResult); err != nil {
			return Result{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		return Result{}, fmt.Errorf("read agent stream: %w", err)
	}
	if !foundResult {
		return Result{}, errors.New("agent stream ended without a result")
	}
	return result, nil
}

func (c *Client) post(ctx context.Context, path string, payload any, accept string) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode agent request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create agent request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", accept)
	request.Header.Set("X-Service-Token", c.serviceToken)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(request.Header))
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call agent service: %w", err)
	}
	return response, nil
}

type streamEvent struct {
	Sequence int             `json:"sequence"`
	Type     string          `json:"type"`
	Payload  json.RawMessage `json:"payload"`
}

func dispatchEvent(event streamEvent, callbacks Callbacks, result *Result, foundResult *bool) error {
	switch event.Type {
	case "status":
		var payload struct {
			Message string `json:"message"`
		}
		return decodeAndCall(event.Payload, &payload, func() error { return call(callbacks.OnStatus, payload.Message) })
	case "agent_step":
		var payload types.AgentStep
		return decodeAndCall(event.Payload, &payload, func() error { return call(callbacks.OnAgentStep, payload) })
	case "sources":
		var payload struct {
			Chunks []types.RetrievalChunk `json:"chunks"`
		}
		return decodeAndCall(event.Payload, &payload, func() error { return call(callbacks.OnSources, payload.Chunks) })
	case "web_sources":
		var payload struct {
			Links []types.ExternalLink `json:"links"`
		}
		return decodeAndCall(event.Payload, &payload, func() error { return call(callbacks.OnWebSources, payload.Links) })
	case "metrics":
		var payload types.RetrievalMetrics
		return decodeAndCall(event.Payload, &payload, func() error { return call(callbacks.OnMetrics, payload) })
	case "delta":
		var payload struct {
			Content string `json:"content"`
		}
		return decodeAndCall(event.Payload, &payload, func() error { return call(callbacks.OnDelta, payload.Content) })
	case "result":
		if err := decodeJSON(bytes.NewReader(event.Payload), result); err != nil {
			return fmt.Errorf("decode agent result: %w", err)
		}
		*foundResult = true
		return nil
	case "error":
		var payload struct {
			Message string `json:"message"`
		}
		if err := decodeJSON(bytes.NewReader(event.Payload), &payload); err != nil {
			return fmt.Errorf("decode agent error: %w", err)
		}
		return errors.New(defaultString(payload.Message, "agent execution failed"))
	default:
		return fmt.Errorf("unsupported agent stream event %q", event.Type)
	}
}

func decodeAndCall(payload json.RawMessage, target any, callback func() error) error {
	if err := decodeJSON(bytes.NewReader(payload), target); err != nil {
		return err
	}
	return callback()
}

func decodeJSON(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected additional JSON value")
		}
		return err
	}
	return nil
}

func call[T any](callback func(T) error, value T) error {
	if callback == nil {
		return nil
	}
	return callback(value)
}

func responseError(response *http.Response) error {
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	return fmt.Errorf("agent service returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
