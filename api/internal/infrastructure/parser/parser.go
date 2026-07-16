package parser

import (
	"context"
	"errors"
	"io"
	"strings"

	"enterprise-rag/api/internal/task"

	"github.com/minio/minio-go/v7"
)

type ParseOptions struct {
	ProcessingMode string
}

func Parse(ctx context.Context, client *minio.Client, bucket, objectName, fileType string, options ParseOptions) (string, []byte, error) {
	object, err := client.GetObject(ctx, bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return "", nil, err
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return "", nil, err
	}

	options.ProcessingMode = task.NormalizeProcessingMode(options.ProcessingMode)

	switch strings.ToLower(fileType) {
	case "md", "markdown":
		return parseMarkdown(data)
	case "txt":
		return parsePlainText(data)
	case "pdf":
		return parsePDF(data, options)
	case "docx":
		return parseDOCX(data, options)
	case "pptx":
		return parsePPTX(data, options)
	case "doc":
		return parseDOC(data, options)
	default:
		return "", nil, errors.New("暂不支持该文件格式")
	}
}
