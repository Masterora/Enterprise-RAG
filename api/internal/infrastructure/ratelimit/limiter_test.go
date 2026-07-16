package ratelimit

import (
	"testing"

	"enterprise-rag/api/internal/config"
)

func TestChatGenerationRoutesShareQuota(t *testing.T) {
	limiter := New(nil, config.RateLimitConf{ChatQuota: 3})

	for _, path := range []string{
		"/api/chat/ask",
		"/api/chat/stream",
		"/api/chat/runs/resume",
	} {
		scope, quota, ok := limiter.policy(path)
		if !ok || scope != "chat" || quota != 3 {
			t.Fatalf("policy(%q) = (%q, %d, %t), want (chat, 3, true)", path, scope, quota, ok)
		}
	}
}
