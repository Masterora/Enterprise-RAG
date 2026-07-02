package auth

import (
	"context"
	"errors"
)

type contextKey string

const userContextKey contextKey = "current_user"

type UserSession struct {
	ID       string
	Username string
	Email    string
}

func WithUser(ctx context.Context, user UserSession) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func CurrentUser(ctx context.Context) (UserSession, error) {
	user, ok := ctx.Value(userContextKey).(UserSession)
	if !ok || user.ID == "" {
		return UserSession{}, errors.New("unauthorized")
	}
	return user, nil
}
