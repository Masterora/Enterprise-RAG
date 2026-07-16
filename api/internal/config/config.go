// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"go.yaml.in/yaml/v3"
)

type Config struct {
	rest.RestConf
	Postgres     PostgresConf
	Redis        RedisConf
	NATS         NATSConf
	MinIO        MinIOConf
	Milvus       MilvusConf
	Worker       WorkerConf
	RateLimit    RateLimitConf
	Chunking     ChunkingConf
	Retrieval    RetrievalConf
	AgentService AgentServiceConf
	Metrics      MetricsConf
	Evaluation   EvaluationConf
	Auth         AuthConf
	Embedding    EmbeddingConf
}

type PostgresConf struct {
	DataSource             string
	MaxConns               int32
	MinConns               int32
	MaxConnLifetimeMinutes int
	MaxConnIdleMinutes     int
	HealthCheckSeconds     int
}

type RedisConf struct {
	Host     string
	Password string
	DB       int
}

type NATSConf struct {
	Url string
}

type MinIOConf struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type MilvusConf struct {
	Address    string
	Collection string
	Dimension  int
	MetricType string
	IndexType  string
}

type WorkerConf struct {
	ParseConcurrency       int
	ChunkConcurrency       int
	EmbeddingConcurrency   int
	DeleteConcurrency      int
	TaskTimeoutSeconds     int
	ShutdownTimeoutSeconds int
}

type RateLimitConf struct {
	PeriodSeconds  int
	ChatQuota      int
	RetrievalQuota int
	UploadQuota    int
}

type ChunkingConf struct {
	Version          string
	Size             int
	Overlap          int
	MinSize          int
	BoundaryLookback int
}

type RetrievalConf struct {
	TopK                   int
	CandidateMultiplier    int
	CandidateLimit         int
	SimilarityThreshold    float64
	RelativeScoreThreshold float64
	MaxCitations           int
	MaxChunksPerDocument   int
	Rerank                 bool
	MaxQueryRunes          int
	MaxSubQueries          int
}

type AgentServiceConf struct {
	URL            string
	ServiceToken   string
	TimeoutSeconds int
}

type MetricsConf struct {
	Enabled bool
	Path    string
}

type EvaluationConf struct {
	MinRecallAtK float64
	MaxLatencyMS int64
}

type AuthConf struct {
	AccessSecret string
	ExpireHours  int
}

type EmbeddingConf struct {
	Provider  string
	Model     string
	Dimension int
	ApiKey    string `json:",optional"`
	BaseURL   string `json:",optional"`
}

type manifest struct {
	Includes []string `yaml:"Includes"`
}

func Load(path string) (Config, error) {
	root, err := readYAMLMap(path)
	if err != nil {
		return Config{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var files manifest
	if err := yaml.Unmarshal(raw, &files); err != nil {
		return Config{}, fmt.Errorf("parse config manifest: %w", err)
	}
	delete(root, "Includes")
	mergedConfig := make(map[string]any)
	baseDir := filepath.Dir(path)
	for _, include := range files.Includes {
		includePath, err := resolveIncludePath(baseDir, include)
		if err != nil {
			return Config{}, err
		}
		included, err := readYAMLMap(includePath)
		if err != nil {
			return Config{}, err
		}
		mergeYAMLMap(mergedConfig, included)
	}
	mergeYAMLMap(mergedConfig, root)
	merged, err := yaml.Marshal(mergedConfig)
	if err != nil {
		return Config{}, fmt.Errorf("merge config: %w", err)
	}
	var result Config
	if err := conf.LoadFromYamlBytes([]byte(os.ExpandEnv(string(merged))), &result); err != nil {
		return Config{}, err
	}
	if err := applyRuntimeOverrides(&result); err != nil {
		return Config{}, err
	}
	if result.Metrics.Enabled {
		result.Metrics.Path = strings.TrimSpace(result.Metrics.Path)
		if result.Metrics.Path == "" {
			result.Metrics.Path = "/metrics"
		}
		if !strings.HasPrefix(result.Metrics.Path, "/") || result.Metrics.Path == "/" || strings.HasPrefix(result.Metrics.Path, "/api/") {
			return Config{}, fmt.Errorf("metrics path must be an absolute non-API path")
		}
	}
	result.AgentService.URL = strings.TrimRight(strings.TrimSpace(result.AgentService.URL), "/")
	result.AgentService.ServiceToken = strings.TrimSpace(result.AgentService.ServiceToken)
	result.Auth.AccessSecret = strings.TrimSpace(result.Auth.AccessSecret)
	result.Redis.Host = strings.TrimSpace(result.Redis.Host)
	result.Redis.Password = strings.TrimSpace(result.Redis.Password)
	result.Chunking.Version = strings.TrimSpace(result.Chunking.Version)
	if result.Chunking.Version == "" {
		result.Chunking.Version = "v1"
	}
	result.Embedding.Provider = strings.ToLower(strings.TrimSpace(result.Embedding.Provider))
	if result.Embedding.Provider == "" {
		result.Embedding.Provider = "openrouter"
	}
	if result.AgentService.URL == "" {
		return Config{}, fmt.Errorf("agent service URL is required")
	}
	if len(result.AgentService.ServiceToken) < 16 {
		return Config{}, fmt.Errorf("agent service token must contain at least 16 characters")
	}
	if len(result.Auth.AccessSecret) < 32 {
		return Config{}, fmt.Errorf("auth access secret must contain at least 32 characters")
	}
	if result.Redis.Host == "" {
		return Config{}, fmt.Errorf("redis host is required")
	}
	if len(result.Redis.Password) < 16 {
		return Config{}, fmt.Errorf("redis password must contain at least 16 characters")
	}
	if result.Worker.TaskTimeoutSeconds < 1 || result.Worker.ShutdownTimeoutSeconds < 1 {
		return Config{}, fmt.Errorf("worker task and shutdown timeouts must be positive")
	}
	if result.RateLimit.PeriodSeconds < 1 || result.RateLimit.ChatQuota < 1 ||
		result.RateLimit.RetrievalQuota < 1 || result.RateLimit.UploadQuota < 1 {
		return Config{}, fmt.Errorf("rate limit values must be positive")
	}
	if result.Postgres.MaxConns < 1 || result.Postgres.MinConns < 0 ||
		result.Postgres.MinConns > result.Postgres.MaxConns ||
		result.Postgres.MaxConnLifetimeMinutes < 1 || result.Postgres.MaxConnIdleMinutes < 1 ||
		result.Postgres.HealthCheckSeconds < 1 {
		return Config{}, fmt.Errorf("invalid postgres pool configuration")
	}
	if result.AgentService.TimeoutSeconds < 1 {
		result.AgentService.TimeoutSeconds = 90
	}
	return result, nil
}

func applyRuntimeOverrides(config *Config) error {
	overrides := []struct {
		name   string
		target *string
	}{
		{"RAG_POSTGRES_DSN", &config.Postgres.DataSource},
		{"RAG_REDIS_HOST", &config.Redis.Host},
		{"RAG_REDIS_PASSWORD", &config.Redis.Password},
		{"RAG_NATS_URL", &config.NATS.Url},
		{"RAG_MINIO_ENDPOINT", &config.MinIO.Endpoint},
		{"RAG_MINIO_ACCESS_KEY", &config.MinIO.AccessKey},
		{"RAG_MINIO_SECRET_KEY", &config.MinIO.SecretKey},
		{"RAG_MILVUS_ADDRESS", &config.Milvus.Address},
		{"RAG_AGENT_URL", &config.AgentService.URL},
		{"RAG_EMBEDDING_BASE_URL", &config.Embedding.BaseURL},
		{"RAG_AUTH_SECRET", &config.Auth.AccessSecret},
		{"OTEL_EXPORTER_OTLP_ENDPOINT", &config.Telemetry.Endpoint},
	}
	for _, override := range overrides {
		if value := strings.TrimSpace(os.Getenv(override.name)); value != "" {
			*override.target = value
		}
	}
	integerOverrides := []struct {
		name   string
		target *int
	}{
		{"RAG_REDIS_DB", &config.Redis.DB},
		{"RAG_RATE_LIMIT_PERIOD_SECONDS", &config.RateLimit.PeriodSeconds},
		{"RAG_RATE_LIMIT_CHAT_QUOTA", &config.RateLimit.ChatQuota},
		{"RAG_RATE_LIMIT_RETRIEVAL_QUOTA", &config.RateLimit.RetrievalQuota},
		{"RAG_RATE_LIMIT_UPLOAD_QUOTA", &config.RateLimit.UploadQuota},
	}
	for _, override := range integerOverrides {
		value := strings.TrimSpace(os.Getenv(override.name))
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("environment variable %s must be an integer: %w", override.name, err)
		}
		*override.target = parsed
	}
	return nil
}

func resolveIncludePath(baseDir, include string) (string, error) {
	include = strings.TrimSpace(include)
	if include == "" || filepath.IsAbs(include) {
		return "", fmt.Errorf("invalid config include %q", include)
	}
	path := filepath.Join(baseDir, filepath.Clean(include))
	relative, err := filepath.Rel(baseDir, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("config include escapes base directory: %q", include)
	}
	return path, nil
}

func readYAMLMap(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	result := make(map[string]any)
	if err := yaml.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return result, nil
}

func mergeYAMLMap(target, source map[string]any) {
	for key, value := range source {
		if sourceMap, ok := value.(map[string]any); ok {
			if targetMap, ok := target[key].(map[string]any); ok {
				mergeYAMLMap(targetMap, sourceMap)
				continue
			}
		}
		target[key] = value
	}
}
