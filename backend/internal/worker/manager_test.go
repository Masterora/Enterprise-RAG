package worker

import (
	"errors"
	"testing"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/svc"
)

func TestNewManagerRejectsInvalidConcurrency(t *testing.T) {
	tests := []struct {
		name   string
		worker config.WorkerConf
	}{
		{
			name: "parse concurrency",
			worker: config.WorkerConf{
				ChunkConcurrency:     1,
				EmbeddingConcurrency: 1,
				DeleteConcurrency:    1,
			},
		},
		{
			name: "chunk concurrency",
			worker: config.WorkerConf{
				ParseConcurrency:     1,
				EmbeddingConcurrency: 1,
				DeleteConcurrency:    1,
			},
		},
		{
			name: "embedding concurrency",
			worker: config.WorkerConf{
				ParseConcurrency:  1,
				ChunkConcurrency:  1,
				DeleteConcurrency: 1,
			},
		},
		{
			name: "delete concurrency",
			worker: config.WorkerConf{
				ParseConcurrency:     1,
				ChunkConcurrency:     1,
				EmbeddingConcurrency: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewManager(&svc.ServiceContext{
				Config: config.Config{Worker: tt.worker},
			})
			if err == nil {
				t.Fatal("NewManager() error = nil, want invalid concurrency error")
			}
		})
	}
}

func TestBuildEmbeddingText(t *testing.T) {
	tests := []struct {
		name  string
		chunk model.DocumentChunk
		want  string
	}{
		{
			name: "includes section heading",
			chunk: model.DocumentChunk{
				Section: "三、在 Jaeger 中应该看什么",
				Content: "1. Trace 列表\n2. Duration\n3. Spans",
			},
			want: "三、在 Jaeger 中应该看什么\n1. Trace 列表\n2. Duration\n3. Spans",
		},
		{
			name: "omits default section",
			chunk: model.DocumentChunk{
				Section: "text",
				Content: "正文",
			},
			want: "正文",
		},
		{
			name: "omits empty section",
			chunk: model.DocumentChunk{
				Content: "正文",
			},
			want: "正文",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildEmbeddingText(tt.chunk); got != tt.want {
				t.Fatalf("buildEmbeddingText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldAutoRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "timeout", err: errors.New("request timeout"), want: true},
		{name: "deadline", err: errors.New("context deadline exceeded"), want: true},
		{name: "connection reset", err: errors.New("connection reset by peer"), want: true},
		{name: "invalid key", err: errors.New("invalid api key"), want: false},
		{name: "unsupported file", err: errors.New("unsupported document type"), want: false},
		{name: "empty", err: errors.New(""), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldAutoRetry(tt.err); got != tt.want {
				t.Fatalf("shouldAutoRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}
