package retrieval

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"sort"
	"strings"
	"unicode"

	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) Search(ctx context.Context, userID, subjectID, query string, topK int) ([]types.RetrievalChunk, error) {
	subjectID = strings.TrimSpace(subjectID)
	query = strings.TrimSpace(query)
	if subjectID == "" || query == "" {
		return nil, errors.New("subject_id and query are required")
	}
	finalTopK := normalizeTopK(topK, s.svcCtx.Config.Milvus.TopK)
	candidateTopK := max(finalTopK, s.svcCtx.Config.Milvus.TopK)

	exists, err := s.svcCtx.SubjectRepo.ExistsAccessible(ctx, subjectID, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("knowledge base not found")
	}

	vectors, err := s.svcCtx.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, errors.New("query embedding is empty")
	}

	chunks, err := s.svcCtx.MilvusStore.Search(ctx, subjectID, vectors[0], candidateTopK)
	if err != nil {
		return nil, err
	}

	vectorChunks := make([]types.RetrievalChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if s.svcCtx.Config.Milvus.MinScore > 0 && chunk.Score < s.svcCtx.Config.Milvus.MinScore {
			continue
		}

		document, err := s.svcCtx.DocumentRepo.GetByID(ctx, chunk.DocID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}

		vectorChunks = append(vectorChunks, types.RetrievalChunk{
			ID:         chunk.ID,
			DocID:      chunk.DocID,
			DocName:    document.Filename,
			SubjectID:  chunk.SubjectID,
			UserID:     chunk.UserID,
			ChunkIndex: chunk.ChunkIndex,
			Page:       chunk.Page,
			Section:    chunk.Section,
			Content:    chunk.Content,
			Score:      chunk.Score,
			RawScore:   chunk.Score,
			Source:     "vector",
		})
	}

	keywords, err := s.keywordSearch(ctx, subjectID, query, candidateTopK)
	if err != nil {
		return nil, err
	}

	normalizeScores(vectorChunks, keywords)
	merged := mergeChunks(keywords, vectorChunks)
	filtered := trimEffectiveChunks(merged, finalTopK)
	return filtered, nil
}

func (s *Service) keywordSearch(ctx context.Context, subjectID, query string, topK int) ([]types.RetrievalChunk, error) {
	if topK <= 0 {
		topK = 5
	}

	chunks, err := s.svcCtx.ChunkRepo.ListBySubject(ctx, subjectID)
	if err != nil {
		return nil, err
	}

	type scoredChunk struct {
		chunk model.DocumentChunk
		score float64
	}
	scored := make([]scoredChunk, 0)
	for _, chunk := range chunks {
		score := keywordScore(query, chunk)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredChunk{chunk: chunk, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	if len(scored) > topK {
		scored = scored[:topK]
	}

	result := make([]types.RetrievalChunk, 0, len(scored))
	for _, item := range scored {
		document, err := s.svcCtx.DocumentRepo.GetByID(ctx, item.chunk.DocID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		result = append(result, types.RetrievalChunk{
			ID:         item.chunk.ID,
			DocID:      item.chunk.DocID,
			DocName:    document.Filename,
			SubjectID:  item.chunk.SubjectID,
			UserID:     item.chunk.UserID,
			ChunkIndex: int64(item.chunk.ChunkIndex),
			Page:       int64(item.chunk.Page),
			Section:    item.chunk.Section,
			Content:    item.chunk.Content,
			Score:      item.score,
			RawScore:   item.score,
			Source:     "keyword",
		})
	}
	return result, nil
}

func keywordScore(query string, chunk model.DocumentChunk) float64 {
	query = normalizeText(query)
	section := normalizeText(chunk.Section)
	content := normalizeText(chunk.Content)
	if query == "" {
		return 0
	}

	var score float64
	if section != "" && strings.Contains(section, query) {
		score += 5
	}
	if section != "" && strings.Contains(content, section) {
		score += 1
	}
	for _, token := range keywordTokens(query) {
		if token == "" {
			continue
		}
		if strings.Contains(section, token) {
			score += 3
		}
		if strings.Contains(content, token) {
			score += 1
		}
	}
	return score
}

func keywordTokens(text string) []string {
	text = normalizeText(text)
	tokens := make([]string, 0)
	var builder strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			continue
		}
		if builder.Len() > 0 {
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
	if len([]rune(token)) < 2 {
		return tokens
	}
	return append(tokens, token)
}

func normalizeText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer("？", "", "?", "", "。", "", ".", "", "，", "", ",", "", "：", "", ":", "")
	return replacer.Replace(text)
}

func mergeChunks(primary, secondary []types.RetrievalChunk) []types.RetrievalChunk {
	seen := make(map[string]struct{})
	merged := make([]types.RetrievalChunk, 0, len(primary)+len(secondary))
	for _, list := range [][]types.RetrievalChunk{primary, secondary} {
		for _, chunk := range list {
			if _, ok := seen[chunk.ID]; ok {
				continue
			}
			seen[chunk.ID] = struct{}{}
			merged = append(merged, chunk)
		}
	}
	return merged
}

func normalizeScores(vectorChunks, keywordChunks []types.RetrievalChunk) {
	normalizeGroup(vectorChunks)
	normalizeGroup(keywordChunks)
}

func normalizeGroup(chunks []types.RetrievalChunk) {
	if len(chunks) == 0 {
		return
	}

	maxScore := chunks[0].RawScore
	minScore := chunks[0].RawScore
	for _, chunk := range chunks[1:] {
		if chunk.RawScore > maxScore {
			maxScore = chunk.RawScore
		}
		if chunk.RawScore < minScore {
			minScore = chunk.RawScore
		}
	}

	for index := range chunks {
		if maxScore == minScore {
			chunks[index].Score = 1
			continue
		}
		chunks[index].Score = (chunks[index].RawScore - minScore) / (maxScore - minScore)
	}
}

func trimEffectiveChunks(chunks []types.RetrievalChunk, limit int) []types.RetrievalChunk {
	if len(chunks) == 0 {
		return chunks
	}
	if limit <= 0 {
		limit = 4
	}

	sort.SliceStable(chunks, func(i, j int) bool {
		if chunks[i].Score == chunks[j].Score {
			return chunks[i].ChunkIndex < chunks[j].ChunkIndex
		}
		return chunks[i].Score > chunks[j].Score
	})

	topScore := chunks[0].Score
	minScore := math.Max(0.35, topScore-0.42)
	seenSignature := make(map[string]struct{})
	seenDocCount := make(map[string]int)
	filtered := make([]types.RetrievalChunk, 0, limit)

	for _, chunk := range chunks {
		if len(filtered) >= limit {
			break
		}
		if chunk.Score < minScore {
			continue
		}
		if seenDocCount[chunk.DocID] >= 2 {
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

	if len(filtered) == 0 {
		filtered = append(filtered, chunks[0])
	}
	return filtered
}

func dedupeSignature(chunk types.RetrievalChunk) string {
	content := normalizeText(chunk.Content)
	runes := []rune(content)
	if len(runes) > 80 {
		content = string(runes[:80])
	}
	return chunk.DocID + "|" + normalizeText(chunk.Section) + "|" + content
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
