package modelsettings

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/repository"

	"github.com/jackc/pgx/v5"
)

const encryptionPurpose = "enterprise-rag/tenant-model-api-key/v1"

var ErrAPIKeyNotConfigured = errors.New("model API key is not configured")

type Status struct {
	Configured bool
	Source     string
	APIKeyHint string
}

type Service struct {
	repo     repository.ModelSettingsRepository
	fallback config.EmbeddingConf
	aead     cipher.AEAD
}

func NewService(repo repository.ModelSettingsRepository, embedding config.EmbeddingConf, accessSecret string) (*Service, error) {
	key := sha256.Sum256([]byte(encryptionPurpose + "\x00" + strings.TrimSpace(accessSecret)))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create model settings cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create model settings AEAD: %w", err)
	}
	return &Service{repo: repo, fallback: embedding, aead: aead}, nil
}

func (s *Service) Status(ctx context.Context, tenantID string) (Status, error) {
	settings, err := s.repo.GetByTenant(ctx, tenantID)
	if err == nil {
		return Status{Configured: true, Source: "page", APIKeyHint: settings.APIKeyHint}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Status{}, err
	}
	if IsConfiguredSecret(s.fallback.ApiKey) {
		return Status{Configured: true, Source: "environment", APIKeyHint: APIKeyHint(s.fallback.ApiKey)}, nil
	}
	return Status{Configured: false, Source: "none"}, nil
}

func (s *Service) ResolveAPIKey(ctx context.Context, tenantID string) (string, error) {
	settings, err := s.repo.GetByTenant(ctx, tenantID)
	if err == nil {
		plaintext, err := s.decrypt(tenantID, settings.APIKeyCiphertext)
		if err != nil {
			return "", fmt.Errorf("decrypt tenant model API key: %w", err)
		}
		return string(plaintext), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if IsConfiguredSecret(s.fallback.ApiKey) {
		return strings.TrimSpace(s.fallback.ApiKey), nil
	}
	return "", ErrAPIKeyNotConfigured
}

func (s *Service) Save(ctx context.Context, tenantID, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if !IsConfiguredSecret(apiKey) {
		return ErrAPIKeyNotConfigured
	}
	ciphertext, err := s.encrypt(tenantID, []byte(apiKey))
	if err != nil {
		return err
	}
	return s.repo.Upsert(ctx, &model.TenantModelSettings{
		TenantID:         tenantID,
		Provider:         providerName(s.fallback.Provider),
		APIKeyCiphertext: ciphertext,
		APIKeyHint:       APIKeyHint(apiKey),
	})
}

func (s *Service) encrypt(tenantID string, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("create model settings nonce: %w", err)
	}
	return s.aead.Seal(nonce, nonce, plaintext, []byte(tenantID)), nil
}

func (s *Service) decrypt(tenantID string, ciphertext []byte) ([]byte, error) {
	nonceSize := s.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("invalid encrypted value")
	}
	return s.aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], []byte(tenantID))
}

func IsConfiguredSecret(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.HasPrefix(value, "replace-with-")
}

func APIKeyHint(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4 {
		return "••••"
	}
	return "••••" + value[len(value)-4:]
}

func providerName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "openrouter"
	}
	return value
}
