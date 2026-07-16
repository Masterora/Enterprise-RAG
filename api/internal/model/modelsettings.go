package model

import "time"

type TenantModelSettings struct {
	TenantID         string
	Provider         string
	APIKeyCiphertext []byte
	APIKeyHint       string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
