package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/infrastructure/ratelimit"
)

type fakeRequestLimiter struct {
	decision ratelimit.Decision
	err      error
	userID   string
	path     string
}

func (f *fakeRequestLimiter) Allow(_ context.Context, userID, path string) (ratelimit.Decision, error) {
	f.userID = userID
	f.path = path
	return f.decision, f.err
}

func TestRateLimitMiddlewareAllowsRequest(t *testing.T) {
	limiter := &fakeRequestLimiter{decision: ratelimit.Decision{Allowed: true}}
	middleware := NewRateLimitMiddleware(limiter)
	called := false
	handler := middleware.Handle(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/api/chat/ask", nil)
	request = request.WithContext(auth.WithUser(request.Context(), auth.UserSession{ID: "user-1", TenantID: "tenant-1"}))
	response := httptest.NewRecorder()

	handler(response, request)

	if !called || response.Code != http.StatusNoContent {
		t.Fatalf("request was not allowed: called=%v status=%d", called, response.Code)
	}
	if limiter.userID != "tenant-1:user-1" || limiter.path != "/api/chat/ask" {
		t.Fatalf("unexpected limiter input: user=%q path=%q", limiter.userID, limiter.path)
	}
}

func TestRateLimitMiddlewareRejectsQuota(t *testing.T) {
	limiter := &fakeRequestLimiter{decision: ratelimit.Decision{Allowed: false, RetryAfter: 7}}
	handler := NewRateLimitMiddleware(limiter).Handle(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler must not be called")
	})
	request := httptest.NewRequest(http.MethodPost, "/api/chat/stream", nil)
	request = request.WithContext(auth.WithUser(request.Context(), auth.UserSession{ID: "user-1", TenantID: "tenant-1"}))
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "7" {
		t.Fatalf("unexpected rate limit response: status=%d retry=%q", response.Code, response.Header().Get("Retry-After"))
	}
}

func TestRateLimitMiddlewareFailsClosedWhenRedisIsUnavailable(t *testing.T) {
	limiter := &fakeRequestLimiter{err: errors.New("redis unavailable")}
	handler := NewRateLimitMiddleware(limiter).Handle(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler must not be called")
	})
	request := httptest.NewRequest(http.MethodPost, "/api/retrieval/search", nil)
	request = request.WithContext(auth.WithUser(request.Context(), auth.UserSession{ID: "user-1", TenantID: "tenant-1"}))
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}
