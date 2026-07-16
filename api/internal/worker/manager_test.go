package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/service/taskqueue"
	"enterprise-rag/api/internal/svc"

	"github.com/nats-io/nats.go"
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

func TestNewManagerRejectsInvalidTimeout(t *testing.T) {
	workerConfig := config.WorkerConf{
		ParseConcurrency:       1,
		ChunkConcurrency:       1,
		EmbeddingConcurrency:   1,
		DeleteConcurrency:      1,
		ShutdownTimeoutSeconds: 1,
	}
	_, err := NewManager(&svc.ServiceContext{Config: config.Config{Worker: workerConfig}})
	if err == nil {
		t.Fatal("NewManager() error = nil, want invalid task timeout error")
	}
}

func TestManagerCloseCancelsInFlightTaskAfterDeadline(t *testing.T) {
	manager, err := NewManager(&svc.ServiceContext{Config: config.Config{Worker: config.WorkerConf{
		ParseConcurrency:       1,
		ChunkConcurrency:       1,
		EmbeddingConcurrency:   1,
		DeleteConcurrency:      1,
		TaskTimeoutSeconds:     1,
		ShutdownTimeoutSeconds: 1,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	finished := make(chan struct{})
	handler := manager.track(func(*nats.Msg) {
		close(started)
		<-manager.ctx.Done()
		close(finished)
	})
	go handler(nats.NewMsg(model.TaskTypeParse))
	<-started

	closeContext, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := manager.Close(closeContext); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close() error = %v, want deadline exceeded", err)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("in-flight task did not observe manager cancellation")
	}
	if err := manager.Close(context.Background()); err != nil {
		t.Fatalf("second Close() error = %v", err)
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
		{name: "durable publish", err: taskqueue.ErrPublish, want: true},
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

func TestDeterministicID(t *testing.T) {
	first := deterministicID("parent-task", model.TaskTypeChunk)
	if second := deterministicID("parent-task", model.TaskTypeChunk); second != first {
		t.Fatalf("deterministicID() changed: first=%q second=%q", first, second)
	}
	if other := deterministicID("parent-task", model.TaskTypeEmbedding); other == first {
		t.Fatalf("deterministicID() did not separate purposes: %q", first)
	}
}

func TestDocumentAtOrBeyond(t *testing.T) {
	if !documentAtOrBeyond(model.DocumentStatusIndexed, model.DocumentStatusParsed) {
		t.Fatal("indexed document should be beyond parsed")
	}
	if documentAtOrBeyond(model.DocumentStatusParsing, model.DocumentStatusChunked) {
		t.Fatal("parsing document must not be beyond chunked")
	}
	if documentAtOrBeyond(model.DocumentStatusFailed, model.DocumentStatusParsed) {
		t.Fatal("terminal failure must not be treated as a completed stage")
	}
}
