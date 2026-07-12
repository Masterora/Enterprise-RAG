package chatflow

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/infrastructure/llm"
)

func GenerateAnswer(ctx context.Context, client llm.Client, reliability config.ReliabilityConf, prompt string) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= normalizedRetries(reliability.MaxRetries); attempt++ {
		callCtx, cancel := generationContext(ctx, reliability.LLMTimeoutSeconds)
		answer, err := client.Generate(callCtx, prompt, false)
		cancel()
		if err == nil {
			return answer, nil
		}
		lastErr = err
		if !isTransientGenerationError(err) || attempt == normalizedRetries(reliability.MaxRetries) {
			break
		}
		if err := waitRetry(ctx, reliability.RetryBackoffMillis, attempt); err != nil {
			return "", err
		}
	}
	return "", lastErr
}

func StreamAnswer(ctx context.Context, client llm.Client, reliability config.ReliabilityConf, prompt string, onDelta func(string) error) error {
	var lastErr error
	for attempt := 0; attempt <= normalizedRetries(reliability.MaxRetries); attempt++ {
		emitted := false
		callCtx, cancel := generationContext(ctx, reliability.LLMTimeoutSeconds)
		err := client.GenerateStream(callCtx, prompt, false, func(delta string) error {
			if delta != "" {
				emitted = true
			}
			return onDelta(delta)
		})
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if emitted || !isTransientGenerationError(err) || attempt == normalizedRetries(reliability.MaxRetries) {
			break
		}
		if err := waitRetry(ctx, reliability.RetryBackoffMillis, attempt); err != nil {
			return err
		}
	}
	return lastErr
}

func generationContext(parent context.Context, timeoutSeconds int) (context.Context, context.CancelFunc) {
	if timeoutSeconds < 1 {
		timeoutSeconds = 45
	}
	return context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
}

func normalizedRetries(value int) int {
	if value < 0 {
		return 0
	}
	if value > 3 {
		return 3
	}
	return value
}

func waitRetry(ctx context.Context, backoffMillis, attempt int) error {
	if backoffMillis < 1 {
		backoffMillis = 300
	}
	timer := time.NewTimer(time.Duration(backoffMillis*(attempt+1)) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isTransientGenerationError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "status=429") || strings.Contains(message, "status=500") ||
		strings.Contains(message, "status=502") || strings.Contains(message, "status=503") ||
		strings.Contains(message, "status=504") || strings.Contains(message, "timeout") ||
		strings.Contains(message, "connection reset") || strings.Contains(message, "unexpected eof")
}

func GenerationFailureKind(err error) string {
	if err == nil {
		return "none"
	}
	message := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(message, "timeout"), strings.Contains(message, "deadline exceeded"):
		return "timeout"
	case strings.Contains(message, "401"), strings.Contains(message, "invalid api key"), strings.Contains(message, "invalid_api_key"):
		return "authentication"
	case strings.Contains(message, "402"), strings.Contains(message, "insufficient credits"):
		return "insufficient_credit"
	case strings.Contains(message, "429"):
		return "rate_limit"
	case strings.Contains(message, "context canceled"):
		return "canceled"
	case strings.Contains(message, "empty") || strings.Contains(message, "unexpected eof"):
		return "incomplete_response"
	default:
		return "provider_error"
	}
}
