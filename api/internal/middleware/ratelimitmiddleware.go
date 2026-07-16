package middleware

import (
	"context"
	"net/http"
	"strconv"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/infrastructure/ratelimit"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type requestLimiter interface {
	Allow(ctx context.Context, userID, path string) (ratelimit.Decision, error)
}

type RateLimitMiddleware struct {
	limiter requestLimiter
}

func NewRateLimitMiddleware(limiter requestLimiter) *RateLimitMiddleware {
	return &RateLimitMiddleware{limiter: limiter}
}

func (m *RateLimitMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.CurrentUser(r.Context())
		if err != nil {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
			return
		}
		decision, err := m.limiter.Allow(r.Context(), user.TenantID+":"+user.ID, r.URL.Path)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("rate limiter failed: path=%s user_id=%s err=%v", r.URL.Path, user.ID, err)
			httpx.WriteJsonCtx(r.Context(), w, http.StatusServiceUnavailable, errorResponse{Message: "service unavailable"})
			return
		}
		if !decision.Allowed {
			w.Header().Set("Retry-After", strconv.Itoa(decision.RetryAfter))
			httpx.WriteJsonCtx(r.Context(), w, http.StatusTooManyRequests, errorResponse{Message: "rate limit exceeded"})
			return
		}
		next(w, r)
	}
}
