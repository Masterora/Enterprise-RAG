package minio

import (
	"context"

	"enterprise-rag/backend/internal/config"

	miniogo "github.com/minio/minio-go/v7"
)

func EnsureBucket(ctx context.Context, client *miniogo.Client, c config.MinIOConf) error {
	exists, err := client.BucketExists(ctx, c.Bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return client.MakeBucket(ctx, c.Bucket, miniogo.MakeBucketOptions{})
}
