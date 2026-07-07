// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"context"
	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/infrastructure/embedding"
	"enterprise-rag/backend/internal/infrastructure/llm"
	milvusinfra "enterprise-rag/backend/internal/infrastructure/milvus"
	minioinfra "enterprise-rag/backend/internal/infrastructure/minio"
	"enterprise-rag/backend/internal/infrastructure/postgres"
	"enterprise-rag/backend/internal/middleware"
	"enterprise-rag/backend/internal/repository"
	pgrepo "enterprise-rag/backend/internal/repository/postgres"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
)

type ServiceContext struct {
	Config         config.Config
	DB             *pgxpool.Pool
	MinIO          *minio.Client
	Nats           *nats.Conn
	Embedder       embedding.Embedder
	LLM            llm.Client
	MilvusStore    *milvusinfra.Store
	AuthMiddleware *middleware.AuthMiddleware
	UserRepo       repository.UserRepository
	SubjectRepo    repository.SubjectRepository
	DocumentRepo   repository.DocumentRepository
	ChunkRepo      repository.ChunkRepository
	IndexTaskRepo  repository.IndexTaskRepository
}

func NewServiceContext(c config.Config) *ServiceContext {
	db, err := pgxpool.New(context.Background(), c.Postgres.DataSource)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	if err := postgres.RunMigrations(context.Background(), db); err != nil {
		log.Fatalf("run postgres migrations: %v", err)
	}

	minioClient, err := minio.New(c.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.MinIO.AccessKey, c.MinIO.SecretKey, ""),
		Secure: c.MinIO.UseSSL,
	})
	if err != nil {
		log.Fatalf("create minio client: %v", err)
	}
	if err := minioinfra.EnsureBucket(context.Background(), minioClient, c.MinIO); err != nil {
		log.Fatalf("initialize minio: %v", err)
	}

	nc, err := nats.Connect(c.NATS.Url)
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	embedder, err := embedding.NewEmbedder(c.Embedding)
	if err != nil {
		log.Fatalf("initialize embedder: %v", err)
	}
	llmClient, err := llm.NewClient(c.LLM)
	if err != nil {
		log.Fatalf("initialize llm: %v", err)
	}
	milvusStore, err := milvusinfra.NewStore(context.Background(), c.Milvus)
	if err != nil {
		log.Fatalf("initialize milvus store: %v", err)
	}

	return &ServiceContext{
		Config:         c,
		DB:             db,
		MinIO:          minioClient,
		Nats:           nc,
		Embedder:       embedder,
		LLM:            llmClient,
		MilvusStore:    milvusStore,
		AuthMiddleware: middleware.NewAuthMiddleware(c.Auth),
		UserRepo:       pgrepo.NewUserRepo(db),
		SubjectRepo:    pgrepo.NewSubjectRepo(db),
		DocumentRepo:   pgrepo.NewDocumentRepo(db),
		ChunkRepo:      pgrepo.NewChunkRepo(db),
		IndexTaskRepo:  pgrepo.NewIndexTaskRepo(db),
	}
}
