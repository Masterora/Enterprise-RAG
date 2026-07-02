package middleware

import (
	"net/http"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/config"

	"github.com/zeromicro/go-zero/rest/httpx"
)

type errorResponse struct {
	Message string `json:"message"`
}

type AuthMiddleware struct {
	config config.AuthConf
}

func NewAuthMiddleware(c config.AuthConf) *AuthMiddleware {
	return &AuthMiddleware{config: c}
}

func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.Header.Get("Authorization"))
		token = strings.TrimPrefix(token, "Bearer ")
		token = strings.TrimSpace(token)
		if token == "" {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
			return
		}

		user, err := auth.ParseToken(m.config.AccessSecret, token)
		if err != nil {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
			return
		}

		next(w, r.WithContext(auth.WithUser(r.Context(), user)))
	}
}
