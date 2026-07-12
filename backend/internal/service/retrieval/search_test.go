package retrieval

import (
	"testing"

	"enterprise-rag/backend/internal/types"
)

func TestRecallAtK(t *testing.T) {
	chunks := []types.RetrievalChunk{
		{ID: "chunk-1", DocID: "doc-1"},
		{ID: "chunk-2", DocID: "doc-2"},
	}

	expected, hits, recall := recallAtK(chunks, []string{"doc-2", "doc-3"}, []string{"chunk-1"})

	if expected != 3 {
		t.Fatalf("expected count = %d, want 3", expected)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
	if recall != float64(2)/float64(3) {
		t.Fatalf("recall = %f, want %f", recall, float64(2)/float64(3))
	}
}

func TestSanitizeRewrittenQuery(t *testing.T) {
	tests := []struct {
		name      string
		original  string
		rewritten string
		want      string
	}{
		{
			name:      "joins multiline output",
			original:  "原问题",
			rewritten: "\"第一行\n第二行\"",
			want:      "第一行 第二行",
		},
		{
			name:      "falls back when model says unknown",
			original:  "原问题",
			rewritten: "无法确定",
			want:      "原问题",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeRewrittenQuery(tt.original, tt.rewritten); got != tt.want {
				t.Fatalf("sanitizeRewrittenQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRerankChunks(t *testing.T) {
	chunks := []types.RetrievalChunk{
		{ID: "chunk-1", ChunkIndex: 1, Content: "完全无关内容", Score: 0.9, Source: "vector"},
		{ID: "chunk-2", ChunkIndex: 2, Section: "引用片段设计", Content: "引用片段用于核查来源", Score: 0.2, Source: "keyword"},
	}

	rerankChunks("为什么显示引用片段", chunks)

	if chunks[0].ID != "chunk-2" {
		t.Fatalf("first chunk = %s, want chunk-2", chunks[0].ID)
	}
}

func TestMergeChunksMarksHybridMatches(t *testing.T) {
	keyword := []types.RetrievalChunk{{ID: "chunk-1", Score: 0.6, RawScore: 4, Source: "keyword"}}
	vector := []types.RetrievalChunk{{ID: "chunk-1", Score: 0.8, RawScore: 0.7, Source: "vector"}}

	merged := mergeChunks(keyword, vector)

	if len(merged) != 1 {
		t.Fatalf("merged count = %d, want 1", len(merged))
	}
	if merged[0].Source != "hybrid" {
		t.Fatalf("source = %q, want hybrid", merged[0].Source)
	}
	if merged[0].Score != 0.9 {
		t.Fatalf("score = %f, want 0.9", merged[0].Score)
	}
}

func TestTrimEffectiveChunks(t *testing.T) {
	tests := []struct {
		name   string
		chunks []types.RetrievalChunk
		limit  int
		wantID []string
	}{
	{
			name: "removes weak and duplicate sources",
			chunks: []types.RetrievalChunk{
				{ID: "strong", DocID: "doc-1", Section: "删除流程", Content: "先删除向量", Score: 0.8},
				{ID: "duplicate", DocID: "doc-1", Section: "删除流程", Content: "先删除向量", Score: 0.75},
				{ID: "weak", DocID: "doc-2", Section: "其他", Content: "无关内容", Score: 0.4},
			},
			limit:  5,
			wantID: []string{"strong"},
		},
		{
			name: "drops all weak chunks instead of forcing fallback",
			chunks: []types.RetrievalChunk{
				{ID: "weak-1", DocID: "doc-1", Section: "其他", Content: "无关内容", Score: 0.43},
				{ID: "weak-2", DocID: "doc-2", Section: "其他", Content: "其他内容", Score: 0.39},
			},
			limit:  5,
			wantID: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := trimEffectiveChunks(tt.chunks, tt.limit)
			if len(filtered) != len(tt.wantID) {
				t.Fatalf("filtered count = %d, want %d", len(filtered), len(tt.wantID))
			}
			for index, wantID := range tt.wantID {
				if filtered[index].ID != wantID {
					t.Fatalf("filtered[%d].ID = %q, want %q", index, filtered[index].ID, wantID)
				}
			}
		})
	}
}
