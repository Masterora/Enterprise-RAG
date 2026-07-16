package retrieval

import (
	"math"
	"sort"
	"strings"

	"enterprise-rag/api/internal/types"
)

type citationPolicy struct {
	limit                int
	absoluteThreshold    float64
	relativeThreshold    float64
	maxChunksPerDocument int
}

func trimEffectiveChunks(chunks []types.RetrievalChunk, policy citationPolicy) []types.RetrievalChunk {
	if len(chunks) == 0 {
		return chunks
	}
	policy = normalizeCitationPolicy(policy)
	sort.SliceStable(chunks, func(i, j int) bool {
		if chunks[i].Score == chunks[j].Score {
			return chunks[i].ChunkIndex < chunks[j].ChunkIndex
		}
		return chunks[i].Score > chunks[j].Score
	})

	minScore := math.Max(policy.absoluteThreshold, chunks[0].Score*policy.relativeThreshold)
	seenSignature := make(map[string]struct{})
	seenDocCount := make(map[string]int)
	filtered := make([]types.RetrievalChunk, 0, policy.limit)
	for _, chunk := range chunks {
		if len(filtered) >= policy.limit || chunk.Score < minScore {
			break
		}
		if seenDocCount[chunk.DocID] >= policy.maxChunksPerDocument {
			continue
		}
		signature := dedupeSignature(chunk)
		if _, ok := seenSignature[signature]; ok {
			continue
		}
		seenSignature[signature] = struct{}{}
		seenDocCount[chunk.DocID]++
		filtered = append(filtered, chunk)
	}
	return mergeAdjacentChunks(filtered)
}

func normalizeCitationPolicy(policy citationPolicy) citationPolicy {
	if policy.limit <= 0 {
		policy.limit = 5
	}
	if policy.absoluteThreshold <= 0 {
		policy.absoluteThreshold = 0.35
	}
	if policy.relativeThreshold <= 0 || policy.relativeThreshold > 1 {
		policy.relativeThreshold = 0.72
	}
	if policy.maxChunksPerDocument < 1 {
		policy.maxChunksPerDocument = 3
	}
	return policy
}

func mergeAdjacentChunks(chunks []types.RetrievalChunk) []types.RetrievalChunk {
	merged := make([]types.RetrievalChunk, 0, len(chunks))
	for _, chunk := range chunks {
		mergedIndex := adjacentChunkIndex(merged, chunk)
		if mergedIndex < 0 {
			merged = append(merged, chunk)
			continue
		}
		if chunk.ChunkIndex < merged[mergedIndex].ChunkIndex {
			merged[mergedIndex].Content = strings.TrimSpace(chunk.Content + "\n" + merged[mergedIndex].Content)
			merged[mergedIndex].ChunkIndex = chunk.ChunkIndex
		} else {
			merged[mergedIndex].Content = strings.TrimSpace(merged[mergedIndex].Content + "\n" + chunk.Content)
		}
		merged[mergedIndex].Score = math.Max(merged[mergedIndex].Score, chunk.Score)
	}
	return merged
}

func adjacentChunkIndex(chunks []types.RetrievalChunk, target types.RetrievalChunk) int {
	for index := range chunks {
		if chunks[index].DocID == target.DocID && chunks[index].Section == target.Section && absInt64(chunks[index].ChunkIndex-target.ChunkIndex) == 1 {
			return index
		}
	}
	return -1
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func dedupeSignature(chunk types.RetrievalChunk) string {
	content := normalizeText(chunk.Content)
	if runes := []rune(content); len(runes) > 80 {
		content = string(runes[:80])
	}
	return chunk.DocID + "|" + normalizeText(chunk.Section) + "|" + content
}

func recallAtK(chunks []types.RetrievalChunk, expectedDocIDs, expectedChunkIDs []string) (int, int, float64) {
	expectedChunks, expectedDocs := normalizeIDSet(expectedChunkIDs), normalizeIDSet(expectedDocIDs)
	expectedCount := len(expectedChunks) + len(expectedDocs)
	if expectedCount == 0 {
		return 0, 0, 0
	}
	hitChunks, hitDocs := make(map[string]struct{}), make(map[string]struct{})
	for _, chunk := range chunks {
		if _, ok := expectedChunks[chunk.ID]; ok {
			hitChunks[chunk.ID] = struct{}{}
		}
		if _, ok := expectedDocs[chunk.DocID]; ok {
			hitDocs[chunk.DocID] = struct{}{}
		}
	}
	hits := len(hitChunks) + len(hitDocs)
	return expectedCount, hits, float64(hits) / float64(expectedCount)
}

func normalizeIDSet(ids []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			set[id] = struct{}{}
		}
	}
	return set
}

func normalizeTopK(requested, fallback int) int {
	if requested > 0 {
		return requested
	}
	if fallback > 0 {
		return fallback
	}
	return 5
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func normalizeMultiplier(value int) int {
	if value < 1 {
		return 1
	}
	return min(value, 10)
}

func normalizeCandidateLimit(value int) int {
	if value < 1 {
		return 50
	}
	return min(value, 200)
}

func normalizeMaxCitations(value int) int {
	if value < 1 {
		return 5
	}
	return min(value, 10)
}
