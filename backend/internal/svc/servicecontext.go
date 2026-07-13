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
	"enterprise-rag/backend/internal/infrastructure/observability"
	"enterprise-rag/backend/internal/infrastructure/postgres"
	"enterprise-rag/backend/internal/middleware"
	"enterprise-rag/backend/internal/repository"
	pgrepo "enterprise-rag/backend/internal/repository/postgres"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/zeromicro/go-zero/rest"
)

type ServiceContext struct {
	Config         config.Config
	DB             *pgxpool.Pool
	MinIO          *minio.Client
	Nats           *nats.Conn
	Embedder       embedding.Embedder
	LLM            llm.Client
	MilvusStore    *milvusinfra.Store
	Metrics        *observability.Metrics
	AuthMiddleware rest.Middleware
	UserRepo       repository.UserRepository
	SubjectRepo    repository.SubjectRepository
	DocumentRepo   repository.DocumentRepository
	ChunkRepo      repository.ChunkRepository
	IndexTaskRepo  repository.IndexTaskRepository
	ChatRepo       repository.ChatRepository
	AdminRepo      repository.AdminRepository
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	db, err := pgxpool.New(context.Background(), c.Postgres.DataSource)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := postgres.RunMigrations(context.Background(), db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run postgres migrations: %w", err)
	}

	minioClient, err := minio.New(c.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.MinIO.AccessKey, c.MinIO.SecretKey, ""),
		Secure: c.MinIO.UseSSL,
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	if err := minioinfra.EnsureBucket(context.Background(), minioClient, c.MinIO); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize minio: %w", err)
	}
	nc, err := nats.Connect(c.NATS.Url)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	embedder, err := embedding.NewEmbedder(c.Embedding)
	if err != nil {
		closeResources(db, nc)
		return nil, fmt.Errorf("initialize embedder: %w", err)
	}
	llmClient, err := llm.NewClient(c.LLM)
	if err != nil {
		closeResources(db, nc)
		return nil, fmt.Errorf("initialize llm: %w", err)
	}
	milvusStore, err := milvusinfra.NewStore(context.Background(), c.Milvus)
	if err != nil {
		closeResources(db, nc)
		return nil, fmt.Errorf("initialize milvus store: %w", err)
	}

	serviceContext := &ServiceContext{
		Config:         c,
		DB:             db,
		MinIO:          minioClient,
		Nats:           nc,
		Embedder:       embedder,
		LLM:            llmClient,
		MilvusStore:    milvusStore,
		DocumentRepo:   pgrepo.NewDocumentRepo(db),
		ChunkRepo:      pgrepo.NewChunkRepo(db),
		IndexTaskRepo:  pgrepo.NewIndexTaskRepo(db),
		AuthMiddleware: middleware.NewAuthMiddleware(c.Auth).Handle,
		UserRepo:       pgrepo.NewUserRepo(db),
		SubjectRepo:    pgrepo.NewSubjectRepo(db),
		ChatRepo:       pgrepo.NewChatRepo(db),
		AdminRepo:      pgrepo.NewAdminRepo(db),
	}
	if c.Metrics.Enabled {
		serviceContext.Metrics = observability.NewMetrics()
	}
	return serviceContext, nil
}

func closeResources(db *pgxpool.Pool, nc *nats.Conn) {
	if nc != nil {
		nc.Close()
	}
	if db != nil {
		db.Close()
	}
}

func (s *ServiceContext) Close() {
	closeResources(s.DB, s.Nats)
}
