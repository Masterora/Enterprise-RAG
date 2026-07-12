package retrieval

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"unicode"

	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/types"
)

func keywordScore(query string, chunk model.DocumentChunk) float64 {
	query, section, content := normalizeText(query), normalizeText(chunk.Section), normalizeText(chunk.Content)
	metadataText := normalizeText(chunkMetadataText(chunk.Metadata))
	if query == "" {
		return 0
	}
	var score float64
	if section != "" && strings.Contains(section, query) {
		score += 5
	}
	if section != "" && strings.Contains(content, section) {
		score++
	}
	for _, token := range keywordTokens(query) {
		if strings.Contains(section, token) {
			score += 3
		}
		if strings.Contains(content, token) {
			score++
		}
		if strings.Contains(metadataText, token) {
			score += 1.5
		}
	}
	return score
}

func chunkMetadataText(raw json.RawMessage) string {
	var metadata model.ChunkMetadata
	if len(raw) == 0 || json.Unmarshal(raw, &metadata) != nil {
		return ""
	}
	parts := append([]string{}, metadata.HeadingPath...)
	parts = append(parts, metadata.Keywords...)
	return strings.Join(append(parts, metadata.Summary), " ")
}

func keywordTokens(text string) []string {
	text = normalizeText(text)
	tokens := make([]string, 0)
	var builder strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		} else if builder.Len() > 0 {
			tokens = appendToken(tokens, builder.String())
			builder.Reset()
		}
	}
	if builder.Len() > 0 {
		tokens = appendToken(tokens, builder.String())
	}
	return tokens
}

func appendToken(tokens []string, token string) []string {
	runes := []rune(token)
	if len(runes) < 2 {
		return tokens
	}
	if containsHan(runes) {
		for i := 0; i+1 < len(runes); i++ {
			tokens = append(tokens, string(runes[i:i+2]))
		}
		if len(runes) <= 12 {
			tokens = append(tokens, token)
		}
		return tokens
	}
	return append(tokens, token)
}

func containsHan(runes []rune) bool {
	for _, r := range runes {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func normalizeText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	return strings.NewReplacer("？", "", "?", "", "。", "", ".", "", "，", "", ",", "", "：", "", ":", "").Replace(text)
}

func mergeChunks(primary, secondary []types.RetrievalChunk) []types.RetrievalChunk {
	indexByID := make(map[string]int)
	merged := make([]types.RetrievalChunk, 0, len(primary)+len(secondary))
	for _, list := range [][]types.RetrievalChunk{primary, secondary} {
		for _, chunk := range list {
			if index, ok := indexByID[chunk.ID]; ok {
				merged[index].Score = clamp01(math.Max(merged[index].Score, chunk.Score) + 0.10)
				merged[index].RawScore = math.Max(merged[index].RawScore, chunk.RawScore)
				merged[index].Source = "hybrid"
				continue
			}
			indexByID[chunk.ID] = len(merged)
			merged = append(merged, chunk)
		}
	}
	return merged
}

func limitCandidates(chunks []types.RetrievalChunk, limit int) []types.RetrievalChunk {
	if limit <= 0 || len(chunks) <= limit {
		return chunks
	}
	sort.SliceStable(chunks, func(i, j int) bool { return chunks[i].Score > chunks[j].Score })
	return chunks[:limit]
}

func normalizeScores(groups ...[]types.RetrievalChunk) {
	for _, chunks := range groups {
		if len(chunks) == 0 {
			continue
		}
		maxScore, minScore := chunks[0].RawScore, chunks[0].RawScore
		for _, chunk := range chunks[1:] {
			maxScore, minScore = math.Max(maxScore, chunk.RawScore), math.Min(minScore, chunk.RawScore)
		}
		for index := range chunks {
			if maxScore == minScore {
				chunks[index].Score = 1
			} else {
				chunks[index].Score = (chunks[index].RawScore - minScore) / (maxScore - minScore)
			}
		}
	}
}

func rerankChunks(query string, chunks []types.RetrievalChunk) {
	for index := range chunks {
		overlap := tokenOverlapScore(query, chunks[index].Section+" "+chunks[index].Content)
		sectionOverlap := tokenOverlapScore(query, chunks[index].Section)
		sourceBoost := 0.0
		if chunks[index].Source == "keyword" {
			sourceBoost = 0.05
		}
		chunks[index].Score = clamp01(chunks[index].Score*0.30 + overlap*0.50 + sectionOverlap*0.15 + sourceBoost)
	}
	sort.SliceStable(chunks, func(i, j int) bool {
		if chunks[i].Score == chunks[j].Score {
			return chunks[i].ChunkIndex < chunks[j].ChunkIndex
		}
		return chunks[i].Score > chunks[j].Score
	})
}

func tokenOverlapScore(query, content string) float64 {
	seen := make(map[string]struct{})
	hits := 0
	content = normalizeText(content)
	for _, token := range keywordTokens(query) {
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		if strings.Contains(content, token) {
			hits++
		}
	}
	if len(seen) == 0 {
		return 0
	}
	return float64(hits) / float64(len(seen))
}

func clamp01(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
