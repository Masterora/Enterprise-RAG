// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"context"
	"enterprise-rag/backend/internal/config"
	minioinfra "enterprise-rag/backend/internal/infrastructure/minio"
	"enterprise-rag/backend/internal/infrastructure/postgres"
	"enterprise-rag/backend/internal/repository"
	pgrepo "enterprise-rag/backend/internal/repository/postgres"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ServiceContext struct {
	Config       config.Config
	MinIO        *minio.Client
	SubjectRepo  repository.SubjectRepository
	DocumentRepo repository.DocumentRepository
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

	return &ServiceContext{
		Config:       c,
		MinIO:        minioClient,
		SubjectRepo:  pgrepo.NewSubjectRepo(db),
		DocumentRepo: pgrepo.NewDocumentRepo(db),
	}
}
