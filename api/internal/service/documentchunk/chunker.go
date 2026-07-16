package documentchunk

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/model"

	"github.com/google/uuid"
)

func Build(document *model.Document, chunking config.ChunkingConf) ([]model.DocumentChunk, error) {
	var metadata model.DocumentMetadata
	if len(document.Metadata) > 0 {
		if err := json.Unmarshal(document.Metadata, &metadata); err != nil {
			return nil, err
		}
	}
	if len(metadata.Segments) == 0 {
		metadata.Segments = []model.ParseSegment{{Content: document.PlainText, BlockType: "paragraph"}}
	}

	size, overlap, minSize, lookback := normalizeConfig(chunking)
	chunks := make([]model.DocumentChunk, 0)
	now := time.Now()
	for _, segment := range metadata.Segments {
		for _, content := range splitSegment(segment.Content, size, overlap, minSize, lookback) {
			contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
			chunkMetadata, err := json.Marshal(model.ChunkMetadata{
				HeadingPath: segment.HeadingPath,
				BlockType:   normalizeBlockType(segment.BlockType),
				Keywords:    extractKeywords(segment.Section, content),
				Summary:     summarize(content),
				SourceType:  document.FileType,
			})
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, model.DocumentChunk{
				ID:                   uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s:%d:%s:%d", document.ID, document.DocumentVersion, contentHash, len(chunks)))).String(),
				TenantID:             document.TenantID,
				DocID:                document.ID,
				SubjectID:            document.SubjectID,
				UserID:               document.UserID,
				ChunkIndex:           len(chunks),
				Content:              content,
				Page:                 segment.Page,
				Section:              segment.Section,
				Metadata:             chunkMetadata,
				TokenCount:           len([]rune(content)),
				DocumentVersion:      document.DocumentVersion,
				ContentHash:          contentHash,
				EmbeddingProvider:    document.EmbeddingProvider,
				EmbeddingModel:       document.EmbeddingModel,
				EmbeddingDimension:   document.EmbeddingDimension,
				ChunkStrategyVersion: document.ChunkStrategyVersion,
				CreatedAt:            now,
				UpdatedAt:            now,
			})
		}
	}
	if len(chunks) == 0 {
		return nil, errors.New("document has no chunkable content")
	}
	return chunks, nil
}

func summarize(content string) string {
	content = strings.Join(strings.Fields(content), " ")
	runes := []rune(content)
	if len(runes) <= 120 {
		return content
	}
	for index := 80; index < 120; index++ {
		switch runes[index] {
		case '。', '！', '？', '.', '!', '?', ';', '；':
			return strings.TrimSpace(string(runes[:index+1]))
		}
	}
	return strings.TrimSpace(string(runes[:120]))
}

func normalizeConfig(value config.ChunkingConf) (int, int, int, int) {
	size := value.Size
	if size < 200 {
		size = 800
	}
	overlap := value.Overlap
	if overlap < 0 || overlap >= size {
		overlap = 120
	}
	minSize := value.MinSize
	if minSize < 1 || minSize >= size {
		minSize = 120
	}
	lookback := value.BoundaryLookback
	if lookback < 0 || lookback >= size {
		lookback = 160
	}
	return size, overlap, minSize, lookback
}

func splitSegment(text string, size, overlap, minSize, lookback int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	result := make([]string, 0, len(runes)/size+1)
	for start := 0; start < len(runes); {
		end := min(start+size, len(runes))
		if end < len(runes) {
			end = findBoundary(runes, start, end, lookback, minSize)
		}
		content := strings.TrimSpace(string(runes[start:end]))
		if content != "" {
			result = append(result, content)
		}
		if end >= len(runes) {
			break
		}
		next := end - overlap
		if next <= start {
			next = end
		}
		start = next
	}
	return result
}

func findBoundary(runes []rune, start, end, lookback, minSize int) int {
	lower := max(start+minSize, end-lookback)
	for index := end; index > lower; index-- {
		switch runes[index-1] {
		case '\n', '。', '！', '？', '.', '!', '?', ';', '；':
			return index
		}
	}
	return end
}

func normalizeBlockType(value string) string {
	if value == "" {
		return "paragraph"
	}
	return value
}

func extractKeywords(section, content string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, 8)
	for _, field := range []string{section, content} {
		for _, token := range strings.FieldsFunc(field, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}) {
			token = strings.TrimSpace(token)
			length := len([]rune(token))
			if length < 2 || length > 32 {
				continue
			}
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			result = append(result, token)
			if len(result) == 8 {
				return result
			}
		}
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
