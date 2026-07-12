package chatflow

import (
	"context"
	"errors"
	"testing"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/types"
)

type retryTestClient struct {
	failures int
	calls    int
	partial  bool
}

func (c *retryTestClient) Generate(context.Context, string, bool) (string, error) {
	c.calls++
	if c.calls <= c.failures {
		return "", errors.New("status=503")
	}
	return "完成", nil
}

func (c *retryTestClient) GenerateStream(_ context.Context, _ string, _ bool, onDelta func(string) error) error {
	c.calls++
	if c.partial {
		if err := onDelta("部分内容"); err != nil {
			return err
		}
	}
	return errors.New("unexpected EOF")
}

func (c *retryTestClient) SearchWeb(context.Context, string) ([]types.ExternalLink, error) {
	return nil, nil
}

func TestGenerateAnswerRetriesTransientFailure(t *testing.T) {
	client := &retryTestClient{failures: 1}
	answer, err := GenerateAnswer(context.Background(), client, config.ReliabilityConf{MaxRetries: 2, RetryBackoffMillis: 1}, "prompt")
	if err != nil || answer != "完成" || client.calls != 2 {
		t.Fatalf("answer=%q calls=%d err=%v", answer, client.calls, err)
	}
}

func TestStreamAnswerDoesNotRetryAfterDelta(t *testing.T) {
	client := &retryTestClient{partial: true}
	err := StreamAnswer(context.Background(), client, config.ReliabilityConf{MaxRetries: 2, RetryBackoffMillis: 1}, "prompt", func(string) error { return nil })
	if err == nil || client.calls != 1 {
		t.Fatalf("calls=%d err=%v", client.calls, err)
	}
}
