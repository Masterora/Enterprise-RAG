package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

func GenerateToken(secret string, expireHours int, user UserSession) (string, error) {
	if secret == "" {
		return "", errors.New("auth secret is required")
	}
	if expireHours <= 0 {
		expireHours = 24
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Username: user.Username,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	return token.SignedString([]byte(secret))
}

func ParseToken(secret, tokenString string) (UserSession, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return UserSession{}, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return UserSession{}, errors.New("invalid token")
	}
	tenantID := claims.TenantID
	if tenantID == "" {
		tenantID = claims.UserID
	}

	return UserSession{
		ID:       claims.UserID,
		TenantID: tenantID,
		Username: claims.Username,
		Email:    claims.Email,
	}, nil
}
