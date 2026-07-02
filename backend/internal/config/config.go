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
	Auth      AuthConf
	LLM       ProviderConf
	Embedding EmbeddingConf
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

type AuthConf struct {
	AccessSecret string
	ExpireHours  int
}

type ProviderConf struct {
	Provider string
	Model    string
	ApiKey   string `json:",optional"`
}

type EmbeddingConf struct {
	Provider  string
	Model     string
	Dimension int
	ApiKey    string `json:",optional"`
}
