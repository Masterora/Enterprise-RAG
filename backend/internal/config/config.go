// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	Postgres  PostgresConf
	Redis     RedisConf
	NATS      NATSConf
	MinIO     MinIOConf
	Milvus    MilvusConf
	Worker    WorkerConf
	Retrieval RetrievalConf
	Auth      AuthConf
	LLM       ProviderConf
	Embedding EmbeddingConf
	Prompt    PromptConf
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
	TopK       int
	MinScore   float64
}

type WorkerConf struct {
	ParseConcurrency     int
	ChunkConcurrency     int
	EmbeddingConcurrency int
	DeleteConcurrency    int
}

type RetrievalConf struct {
	QueryRewrite          bool
	Rerank                bool
	CandidateMultiplier   int
	RewriteTimeoutSeconds int
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
	AnswerTemplate         string `json:",optional"`
	WebSearchTemplate      string `json:",optional"`
	OverviewPolishTemplate string `json:",optional"`
	QueryRewriteTemplate   string `json:",optional"`
}
