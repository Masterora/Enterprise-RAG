package model

import "time"

type User struct {
	ID           string
	TenantID     string
	Username     string
	Nickname     string
	Email        string
	Language     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
