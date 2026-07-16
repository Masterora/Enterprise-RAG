package documentchunk

import (
	"encoding/json"
	"strings"
	"testing"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/model"
)

func TestBuildPreservesStructureAndSentenceBoundary(t *testing.T) {
	metadata, err := json.Marshal(model.DocumentMetadata{Segments: []model.ParseSegment{{
		Section:     "删除策略 / 删除流程",
		HeadingPath: []string{"删除策略", "删除流程"},
		BlockType:   "paragraph",
		Content:     strings.Repeat("删除向量后再删除原文，可以避免孤儿向量。", 30),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	document := &model.Document{ID: "doc-1", SubjectID: "subject-1", UserID: "user-1", Metadata: metadata}

	chunks, err := Build(document, config.ChunkingConf{Size: 200, Overlap: 30, MinSize: 40, BoundaryLookback: 60})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunk count = %d, want at least 2", len(chunks))
	}
	if !strings.HasSuffix(chunks[0].Content, "。") {
		t.Fatalf("first chunk does not end at sentence boundary: %q", chunks[0].Content)
	}
	var chunkMetadata model.ChunkMetadata
	if err := json.Unmarshal(chunks[0].Metadata, &chunkMetadata); err != nil {
		t.Fatal(err)
	}
	if len(chunkMetadata.HeadingPath) != 2 || chunkMetadata.HeadingPath[1] != "删除流程" {
		t.Fatalf("heading path = %#v", chunkMetadata.HeadingPath)
	}
}
