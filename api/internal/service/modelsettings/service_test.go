package modelsettings

import (
	"context"
	"testing"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/model"

	"github.com/jackc/pgx/v5"
)

type memoryRepo struct {
	settings map[string]*model.TenantModelSettings
}

func (r *memoryRepo) GetByTenant(_ context.Context, tenantID string) (*model.TenantModelSettings, error) {
	settings, ok := r.settings[tenantID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	copy := *settings
	copy.APIKeyCiphertext = append([]byte(nil), settings.APIKeyCiphertext...)
	return &copy, nil
}

func (r *memoryRepo) Upsert(_ context.Context, settings *model.TenantModelSettings) error {
	copy := *settings
	copy.APIKeyCiphertext = append([]byte(nil), settings.APIKeyCiphertext...)
	r.settings[settings.TenantID] = &copy
	return nil
}

func TestServiceEncryptsAndIsolatesTenantAPIKeys(t *testing.T) {
	repo := &memoryRepo{settings: make(map[string]*model.TenantModelSettings)}
	service, err := NewService(repo, config.EmbeddingConf{Provider: "openrouter"}, "a-secret-long-enough-for-this-test")
	if err != nil {
		t.Fatal(err)
	}
	const apiKey = "sk-or-v1-tenant-secret"
	if err := service.Save(context.Background(), "tenant-a", apiKey); err != nil {
		t.Fatal(err)
	}
	stored := repo.settings["tenant-a"]
	if string(stored.APIKeyCiphertext) == apiKey {
		t.Fatal("API key was stored as plaintext")
	}
	resolved, err := service.ResolveAPIKey(context.Background(), "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != apiKey {
		t.Fatalf("resolved key = %q", resolved)
	}
	if _, err := service.ResolveAPIKey(context.Background(), "tenant-b"); err != ErrAPIKeyNotConfigured {
		t.Fatalf("tenant-b error = %v, want ErrAPIKeyNotConfigured", err)
	}
	status, err := service.Status(context.Background(), "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Configured || status.Source != "page" || status.APIKeyHint != "••••cret" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestServiceUsesEnvironmentFallback(t *testing.T) {
	repo := &memoryRepo{settings: make(map[string]*model.TenantModelSettings)}
	service, err := NewService(repo, config.EmbeddingConf{ApiKey: "sk-or-v1-environment"}, "a-secret-long-enough-for-this-test")
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.Status(context.Background(), "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Configured || status.Source != "environment" || status.APIKeyHint != "••••ment" {
		t.Fatalf("unexpected status: %+v", status)
	}
}
