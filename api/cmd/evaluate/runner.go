package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type retrievalMetrics struct {
	RecallAtK        float64 `json:"recall_at_k"`
	RouteCorrect     bool    `json:"route_correct"`
	EvaluationPassed bool    `json:"evaluation_passed"`
	LatencyMS        int64   `json:"latency_ms"`
	CandidateCount   int     `json:"candidate_count"`
	ReturnedCount    int     `json:"returned_count"`
	CitationCount    int     `json:"citation_count"`
	Answered         bool    `json:"answered"`
	OutcomeCorrect   bool    `json:"outcome_correct"`
}

type apiResponse struct {
	Metrics retrievalMetrics `json:"metrics"`
}

type caseResult struct {
	Name            string
	ExpectedRecall  bool
	ExpectedRoute   bool
	ExpectedOutcome bool
	Metrics         retrievalMetrics
	Duration        time.Duration
	Err             error
}

type runner struct {
	baseURL     string
	token       string
	concurrency int
	client      *http.Client
}

func newRunner(baseURL, token string, concurrency int, timeout time.Duration) (*runner, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	token = strings.TrimSpace(token)
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	if token == "" {
		return nil, fmt.Errorf("RAG_API_TOKEN is required")
	}
	if concurrency < 1 {
		return nil, fmt.Errorf("concurrency must be positive")
	}
	return &runner{
		baseURL:     baseURL,
		token:       token,
		concurrency: concurrency,
		client:      &http.Client{Timeout: timeout},
	}, nil
}

func (r *runner) run(ctx context.Context, s suite) []caseResult {
	results := make([]caseResult, len(s.Cases))
	jobs := make(chan int)
	var workers sync.WaitGroup

	for range min(r.concurrency, len(s.Cases)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				results[index] = r.runCase(ctx, s, s.Cases[index])
			}
		}()
	}

	for index := range s.Cases {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
	return results
}

func (r *runner) runCase(ctx context.Context, s suite, tc testCase) caseResult {
	result := caseResult{
		Name:            tc.Name,
		ExpectedRecall:  len(tc.ExpectedDocIDs)+len(tc.ExpectedChunkIDs) > 0,
		ExpectedRoute:   strings.TrimSpace(tc.ExpectedRoute) != "",
		ExpectedOutcome: strings.TrimSpace(tc.ExpectedOutcome) != "",
	}
	payload := map[string]any{
		"subject_id":         s.SubjectID,
		"query":              tc.Query,
		"top_k":              s.TopK,
		"expected_doc_ids":   tc.ExpectedDocIDs,
		"expected_chunk_ids": tc.ExpectedChunkIDs,
		"expected_route":     tc.ExpectedRoute,
	}
	endpoint := "/api/retrieval/search"
	if s.Mode == modeAnswer {
		endpoint = "/api/chat/ask"
		payload["expected_outcome"] = tc.ExpectedOutcome
		payload["llm_provider"] = s.LLMProvider
		payload["llm_model"] = s.LLMModel
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		result.Err = fmt.Errorf("encode request: %w", err)
		return result
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+endpoint, bytes.NewReader(requestBody))
	if err != nil {
		result.Err = fmt.Errorf("create request: %w", err)
		return result
	}
	request.Header.Set("Authorization", "Bearer "+r.token)
	request.Header.Set("Content-Type", "application/json")

	startedAt := time.Now()
	response, err := r.client.Do(request)
	result.Duration = time.Since(startedAt)
	if err != nil {
		result.Err = fmt.Errorf("request failed: %w", err)
		return result
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		result.Err = fmt.Errorf("read response: %w", err)
		return result
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		result.Err = fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
		return result
	}

	var decoded apiResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		result.Err = fmt.Errorf("decode response: %w", err)
		return result
	}
	result.Metrics = decoded.Metrics
	return result
}
