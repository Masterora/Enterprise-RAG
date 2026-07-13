// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"go.yaml.in/yaml/v3"
)

type Config struct {
	rest.RestConf
	Postgres    PostgresConf
	Redis       RedisConf
	NATS        NATSConf
	MinIO       MinIOConf
	Milvus      MilvusConf
	Worker      WorkerConf
	Chunking    ChunkingConf
	Retrieval   RetrievalConf
	Reliability ReliabilityConf
	Agent       AgentConf
	Metrics     MetricsConf
	Evaluation  EvaluationConf
	Auth        AuthConf
	LLM         ProviderConf
	Embedding   EmbeddingConf
	Prompt      PromptConf
}

type PostgresConf struct {
	DataSource string
}

type RedisConf struct {
	Host string
	Pass string
	Type string
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
	ParseConcurrency     int
	ChunkConcurrency     int
	EmbeddingConcurrency int
	DeleteConcurrency    int
}

type ChunkingConf struct {
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
	QueryRewrite           bool
	Rerank                 bool
	RewriteTimeoutSeconds  int
	MaxQueryRunes          int
	MaxSubQueries          int
}

type ReliabilityConf struct {
	LLMTimeoutSeconds  int
	MaxRetries         int
	RetryBackoffMillis int
}

type AgentConf struct {
	Enabled             bool
	MaxIterations       int
	MaxTools            int
	MaxTotalTools       int
	TimeoutSeconds      int
	ToolTimeoutSeconds  int
	MaxQuestionRunes    int
	MaxArgumentRunes    int
	MaxObservationRunes int
	MaxStepDetailRunes  int
	ParallelTools       bool
	EnabledTools        []string
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

type ProviderConf struct {
	Provider string
	Model    string
	ApiKey   string `json:",optional"`
	BaseURL  string `json:",optional"`
}

type EmbeddingConf struct {
	Provider  string
	Model     string
	Dimension int
	ApiKey    string `json:",optional"`
	BaseURL   string `json:",optional"`
}

type PromptConf struct {
	QueryRewriteTemplate string `json:",optional"`
	AgentPlanTemplate    string `json:",optional"`
	AgentAnswerTemplate  string `json:",optional"`
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
	if result.Metrics.Enabled {
		result.Metrics.Path = strings.TrimSpace(result.Metrics.Path)
		if result.Metrics.Path == "" {
			result.Metrics.Path = "/metrics"
		}
		if !strings.HasPrefix(result.Metrics.Path, "/") || result.Metrics.Path == "/" || strings.HasPrefix(result.Metrics.Path, "/api/") {
			return Config{}, fmt.Errorf("metrics path must be an absolute non-API path")
		}
	}
	return result, nil
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
