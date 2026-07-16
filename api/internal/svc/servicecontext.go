// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"enterprise-rag/api/internal/config"
	agentinfra "enterprise-rag/api/internal/infrastructure/agent"
	"enterprise-rag/api/internal/infrastructure/embedding"
	milvusinfra "enterprise-rag/api/internal/infrastructure/milvus"
	minioinfra "enterprise-rag/api/internal/infrastructure/minio"
	"enterprise-rag/api/internal/infrastructure/observability"
	"enterprise-rag/api/internal/infrastructure/postgres"
	"enterprise-rag/api/internal/infrastructure/ratelimit"
	"enterprise-rag/api/internal/middleware"
	"enterprise-rag/api/internal/repository"
	pgrepo "enterprise-rag/api/internal/repository/postgres"
	modelsettingssvc "enterprise-rag/api/internal/service/modelsettings"
	"enterprise-rag/api/internal/service/runcontrol"
	"enterprise-rag/api/internal/service/taskqueue"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/rest"
)

type ServiceContext struct {
	Config              config.Config
	DB                  *pgxpool.Pool
	Redis               *redis.Client
	MinIO               *minio.Client
	minioTransport      *http.Transport
	Nats                *nats.Conn
	JetStream           nats.JetStreamContext
	Embedder            embedding.Embedder
	MilvusStore         *milvusinfra.Store
	Agent               *agentinfra.Client
	Metrics             *observability.Metrics
	AuthMiddleware      rest.Middleware
	RateLimitMiddleware rest.Middleware
	UserRepo            repository.UserRepository
	SubjectRepo         repository.SubjectRepository
	DocumentRepo        repository.DocumentRepository
	ChunkRepo           repository.ChunkRepository
	IndexTaskRepo       repository.IndexTaskRepository
	OutboxRepo          repository.OutboxRepository
	ChatRepo            repository.ChatRepository
	AdminRepo           repository.AdminRepository
	RunRepo             repository.RunRepository
	ModelSettingsRepo   repository.ModelSettingsRepository
	ModelSettings       *modelsettingssvc.Service
	RunController       *runcontrol.Controller
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	startupCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(c.Postgres.DataSource)
	if err != nil {
		return nil, fmt.Errorf("parse postgres configuration: %w", err)
	}
	poolConfig.MaxConns = c.Postgres.MaxConns
	poolConfig.MinConns = c.Postgres.MinConns
	poolConfig.MaxConnLifetime = time.Duration(c.Postgres.MaxConnLifetimeMinutes) * time.Minute
	poolConfig.MaxConnIdleTime = time.Duration(c.Postgres.MaxConnIdleMinutes) * time.Minute
	poolConfig.HealthCheckPeriod = time.Duration(c.Postgres.HealthCheckSeconds) * time.Second
	db, err := pgxpool.NewWithConfig(startupCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := db.Ping(startupCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if err := postgres.RunMigrations(startupCtx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run postgres migrations: %w", err)
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:         c.Redis.Host,
		Password:     c.Redis.Password,
		DB:           c.Redis.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolTimeout:  3 * time.Second,
	})
	if err := redisClient.Ping(startupCtx).Err(); err != nil {
		_ = redisClient.Close()
		db.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	minioTransport, err := minio.DefaultTransport(c.MinIO.UseSSL)
	if err != nil {
		closeResources(db, redisClient, nil, nil, nil, nil)
		return nil, fmt.Errorf("create minio transport: %w", err)
	}
	minioClient, err := minio.New(c.MinIO.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(c.MinIO.AccessKey, c.MinIO.SecretKey, ""),
		Secure:    c.MinIO.UseSSL,
		Transport: minioTransport,
	})
	if err != nil {
		closeResources(db, redisClient, nil, minioTransport, nil, nil)
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	if err := minioinfra.EnsureBucket(startupCtx, minioClient, c.MinIO); err != nil {
		closeResources(db, redisClient, nil, minioTransport, nil, nil)
		return nil, fmt.Errorf("initialize minio: %w", err)
	}
	nc, err := nats.Connect(
		c.NATS.Url,
		nats.Name("enterprise-rag-api"),
		nats.Timeout(5*time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DrainTimeout(time.Duration(c.Worker.ShutdownTimeoutSeconds)*time.Second),
	)
	if err != nil {
		closeResources(db, redisClient, nil, minioTransport, nil, nil)
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	jetStream, err := nc.JetStream()
	if err != nil {
		closeResources(db, redisClient, nc, minioTransport, nil, nil)
		return nil, fmt.Errorf("initialize NATS JetStream: %w", err)
	}
	if err := taskqueue.EnsureStream(startupCtx, jetStream); err != nil {
		closeResources(db, redisClient, nc, minioTransport, nil, nil)
		return nil, err
	}
	modelSettingsRepo := pgrepo.NewModelSettingsRepo(db)
	modelSettings, err := modelsettingssvc.NewService(modelSettingsRepo, c.Embedding, c.Auth.AccessSecret)
	if err != nil {
		closeResources(db, redisClient, nc, minioTransport, nil, nil)
		return nil, fmt.Errorf("initialize model settings: %w", err)
	}
	embedder, err := embedding.NewEmbedder(c.Embedding, modelSettings)
	if err != nil {
		closeResources(db, redisClient, nc, minioTransport, nil, nil)
		return nil, fmt.Errorf("initialize embedder: %w", err)
	}
	milvusStore, err := milvusinfra.NewStore(startupCtx, c.Milvus)
	if err != nil {
		closeResources(db, redisClient, nc, minioTransport, embedder, nil)
		return nil, fmt.Errorf("initialize milvus store: %w", err)
	}
	rateLimiter := ratelimit.New(redisClient, c.RateLimit)

	serviceContext := &ServiceContext{
		Config:              c,
		DB:                  db,
		Redis:               redisClient,
		MinIO:               minioClient,
		minioTransport:      minioTransport,
		Nats:                nc,
		JetStream:           jetStream,
		Embedder:            embedder,
		MilvusStore:         milvusStore,
		Agent:               agentinfra.NewClient(c.AgentService),
		DocumentRepo:        pgrepo.NewDocumentRepo(db),
		ChunkRepo:           pgrepo.NewChunkRepo(db),
		IndexTaskRepo:       pgrepo.NewIndexTaskRepo(db),
		OutboxRepo:          pgrepo.NewOutboxRepo(db),
		AuthMiddleware:      middleware.NewAuthMiddleware(c.Auth).Handle,
		RateLimitMiddleware: middleware.NewRateLimitMiddleware(rateLimiter).Handle,
		UserRepo:            pgrepo.NewUserRepo(db),
		SubjectRepo:         pgrepo.NewSubjectRepo(db),
		ChatRepo:            pgrepo.NewChatRepo(db),
		AdminRepo:           pgrepo.NewAdminRepo(db),
		RunRepo:             pgrepo.NewRunRepo(db),
		ModelSettingsRepo:   modelSettingsRepo,
		ModelSettings:       modelSettings,
		RunController:       runcontrol.New(),
	}
	if c.Metrics.Enabled {
		serviceContext.Metrics = observability.NewMetrics()
	}
	return serviceContext, nil
}

func closeResources(
	db *pgxpool.Pool,
	redisClient *redis.Client,
	nc *nats.Conn,
	minioTransport *http.Transport,
	embedder embedding.Embedder,
	milvusStore *milvusinfra.Store,
) {
	if milvusStore != nil {
		milvusStore.Close()
	}
	if embedder != nil {
		embedder.Close()
	}
	if minioTransport != nil {
		minioTransport.CloseIdleConnections()
	}
	if nc != nil {
		nc.Close()
	}
	if redisClient != nil {
		_ = redisClient.Close()
	}
	if db != nil {
		db.Close()
	}
}

func (s *ServiceContext) Close() {
	if s.Agent != nil {
		s.Agent.Close()
	}
	if s.Nats != nil {
		if err := s.Nats.Drain(); err != nil {
			s.Nats.Close()
		}
	}
	if s.MilvusStore != nil {
		s.MilvusStore.Close()
	}
	if s.Embedder != nil {
		s.Embedder.Close()
	}
	if s.minioTransport != nil {
		s.minioTransport.CloseIdleConnections()
	}
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
	if s.DB != nil {
		s.DB.Close()
	}
}

func (s *ServiceContext) Ready(ctx context.Context) error {
	if err := s.DB.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := s.Redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	if !s.Nats.IsConnected() {
		return fmt.Errorf("nats: connection is not ready")
	}
	if err := s.Nats.FlushWithContext(ctx); err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	if _, err := s.JetStream.AccountInfo(nats.Context(ctx)); err != nil {
		return fmt.Errorf("nats JetStream: %w", err)
	}
	bucketExists, err := s.MinIO.BucketExists(ctx, s.Config.MinIO.Bucket)
	if err != nil {
		return fmt.Errorf("minio: %w", err)
	}
	if !bucketExists {
		return fmt.Errorf("minio: bucket %q does not exist", s.Config.MinIO.Bucket)
	}
	if err := s.MilvusStore.Ready(ctx); err != nil {
		return fmt.Errorf("milvus: %w", err)
	}
	if err := s.Agent.Ready(ctx); err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	return nil
}
