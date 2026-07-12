package retrieval

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/infrastructure/llm"
	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

type SearchOptions struct {
	TopK             int
	ExpectedDocIDs   []string
	ExpectedChunkIDs []string
	LLMProvider      string
	LLMModel         string
	OnStage          func(string) error
}

func notifyStage(callback func(string) error, message string) error {
	if callback == nil {
		return nil
	}
	return callback(message)
}

func (s *Service) Search(ctx context.Context, userID, subjectID, query string, topK int) ([]types.RetrievalChunk, error) {
	chunks, _, err := s.SearchWithOptions(ctx, userID, subjectID, query, SearchOptions{TopK: topK})
	return chunks, err
}

func (s *Service) SearchWithOptions(ctx context.Context, userID, subjectID, query string, options SearchOptions) ([]types.RetrievalChunk, types.RetrievalMetrics, error) {
	subjectID = strings.TrimSpace(subjectID)
	query = strings.TrimSpace(query)
	if subjectID == "" || query == "" {
		return nil, types.RetrievalMetrics{}, errors.New("subject_id and query are required")
	}
	finalTopK := normalizeTopK(options.TopK, s.svcCtx.Config.Milvus.TopK)
	candidateTopK := max(finalTopK, s.svcCtx.Config.Milvus.TopK)
	if s.svcCtx.Config.Retrieval.Rerank {
		candidateTopK *= normalizeMultiplier(s.svcCtx.Config.Retrieval.CandidateMultiplier)
	}
	candidateTopK = min(candidateTopK, 50)

	metrics := types.RetrievalMetrics{
		OriginalQuery: query,
		SearchQuery:   query,
		TopK:          finalTopK,
		Reranked:      s.svcCtx.Config.Retrieval.Rerank,
	}

	exists, err := s.svcCtx.SubjectRepo.ExistsAccessible(ctx, subjectID, userID)
	if err != nil {
		return nil, metrics, err
	}
	if !exists {
		return nil, metrics, errors.New("knowledge base not found")
	}

	searchQuery := query
	if s.svcCtx.Config.Retrieval.QueryRewrite {
		if err := notifyStage(options.OnStage, "正在改写检索问题..."); err != nil {
			return nil, metrics, err
		}
		rewritten, err := s.rewriteQuery(ctx, query, options.LLMProvider, options.LLMModel)
		if err != nil {
			logx.WithContext(ctx).Errorf("query rewrite failed, fallback to original query: subject_id=%s err=%v", subjectID, err)
			if err := notifyStage(options.OnStage, "检索问题改写失败，使用原问题检索"); err != nil {
				return nil, metrics, err
			}
		} else if rewritten != "" && rewritten != query {
			searchQuery = rewritten
			metrics.SearchQuery = rewritten
			metrics.QueryRewritten = true
			if err := notifyStage(options.OnStage, "检索问题已改写"); err != nil {
				return nil, metrics, err
			}
		} else {
			if err := notifyStage(options.OnStage, "未改写检索问题，使用原问题检索"); err != nil {
				return nil, metrics, err
			}
		}
	} else if err := notifyStage(options.OnStage, "未启用检索问题改写，使用原问题检索"); err != nil {
		return nil, metrics, err
	}

	if err := notifyStage(options.OnStage, "正在生成问题向量..."); err != nil {
		return nil, metrics, err
	}
	vectors, err := s.svcCtx.Embedder.Embed(ctx, []string{searchQuery})
	if err != nil {
		return nil, metrics, err
	}
	if len(vectors) == 0 {
		return nil, metrics, errors.New("query embedding is empty")
	}

	if err := notifyStage(options.OnStage, "正在检索 Milvus 向量库..."); err != nil {
		return nil, metrics, err
	}
	chunks, err := s.svcCtx.MilvusStore.Search(ctx, subjectID, vectors[0], candidateTopK)
	if err != nil {
		return nil, metrics, err
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
			return nil, metrics, err
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

	if err := notifyStage(options.OnStage, "正在执行关键词召回..."); err != nil {
		return nil, metrics, err
	}
	keywords, err := s.keywordSearch(ctx, subjectID, joinQueries(query, searchQuery), candidateTopK)
	if err != nil {
		return nil, metrics, err
	}

	if err := notifyStage(options.OnStage, "正在合并召回结果..."); err != nil {
		return nil, metrics, err
	}
	normalizeScores(vectorChunks, keywords)
	merged := mergeChunks(keywords, vectorChunks)
	metrics.CandidateCount = len(merged)
	if s.svcCtx.Config.Retrieval.Rerank {
		if err := notifyStage(options.OnStage, "正在重排候选片段..."); err != nil {
			return nil, metrics, err
		}
		rerankChunks(query, merged)
	} else if err := notifyStage(options.OnStage, "未执行重排"); err != nil {
		return nil, metrics, err
	}
	if err := notifyStage(options.OnStage, "正在裁剪引用片段..."); err != nil {
		return nil, metrics, err
	}
	filtered := trimEffectiveChunks(merged, finalTopK)
	metrics.ReturnedCount = len(filtered)
	metrics.ExpectedCount, metrics.RecallHitCount, metrics.RecallAtK = recallAtK(filtered, options.ExpectedDocIDs, options.ExpectedChunkIDs)
	logx.WithContext(ctx).Infof("retrieval finished: subject_id=%s top_k=%d candidates=%d returned=%d rewrite=%t rerank=%t recall_at_k=%.4f expected=%d hits=%d",
		subjectID, finalTopK, metrics.CandidateCount, metrics.ReturnedCount, metrics.QueryRewritten, metrics.Reranked, metrics.RecallAtK, metrics.ExpectedCount, metrics.RecallHitCount)
	return filtered, metrics, nil
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

func (s *Service) rewriteQuery(ctx context.Context, query, provider, model string) (string, error) {
	timeout := s.svcCtx.Config.Retrieval.RewriteTimeoutSeconds
	if timeout <= 0 {
		timeout = 6
	}
	rewriteCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	prompt := strings.TrimSpace(s.svcCtx.Config.Prompt.QueryRewriteTemplate)
	if prompt == "" {
		prompt = `请把下面的问题改写成更适合知识库检索的查询句。
要求：
1. 保留原问题含义，不要回答问题。
2. 补充同义表达和关键实体，但不要编造新事实。
3. 只输出一行检索查询，不要解释，长度控制在 80 字以内。

原问题：{{question}}`
	}
	prompt = strings.ReplaceAll(prompt, "{{question}}", query)

	rewriteLLM, err := s.resolveRewriteLLM(provider, model)
	if err != nil {
		return "", err
	}
	rewritten, err := rewriteLLM.Generate(rewriteCtx, prompt, false)
	if err != nil {
		return "", err
	}
	return sanitizeRewrittenQuery(query, rewritten), nil
}

func (s *Service) resolveRewriteLLM(provider, model string) (llm.Client, error) {
	override := config.ProviderConf{
		Provider: strings.TrimSpace(provider),
		Model:    strings.TrimSpace(model),
		ApiKey:   s.svcCtx.Config.LLM.ApiKey,
		BaseURL:  s.svcCtx.Config.LLM.BaseURL,
	}
	if override.Provider == "" {
		override.Provider = s.svcCtx.Config.LLM.Provider
	}
	if override.Model == "" {
		override.Model = s.svcCtx.Config.LLM.Model
	}
	if strings.EqualFold(override.Provider, strings.TrimSpace(s.svcCtx.Config.LLM.Provider)) &&
		override.Model == strings.TrimSpace(s.svcCtx.Config.LLM.Model) {
		return s.svcCtx.LLM, nil
	}
	return llm.NewClient(override)
}

func sanitizeRewrittenQuery(original, rewritten string) string {
	rewritten = strings.TrimSpace(rewritten)
	rewritten = strings.Trim(rewritten, "`\"'“”‘’")
	rewritten = strings.ReplaceAll(rewritten, "\n", " ")
	rewritten = strings.Join(strings.Fields(rewritten), " ")
	if rewritten == "" || strings.EqualFold(rewritten, "无法确定") {
		return original
	}
	runes := []rune(rewritten)
	if len(runes) > 120 {
		rewritten = string(runes[:120])
	}
	return rewritten
}

func joinQueries(original, rewritten string) string {
	if strings.TrimSpace(original) == strings.TrimSpace(rewritten) {
		return original
	}
	return original + " " + rewritten
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
	replacer := strings.NewReplacer("？", "", "?", "", "。", "", ".", "", "，", "", ",", "", "：", "", ":", "")
	return replacer.Replace(text)
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

func rerankChunks(query string, chunks []types.RetrievalChunk) {
	if len(chunks) == 0 {
		return
	}

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
	queryTokens := keywordTokens(query)
	if len(queryTokens) == 0 {
		return 0
	}
	content = normalizeText(content)
	hits := 0
	seen := make(map[string]struct{})
	for _, token := range queryTokens {
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		if strings.Contains(content, token) {
			hits++
		}
	}
	return float64(hits) / float64(len(seen))
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
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
	minScore := math.Max(0.50, topScore*0.72)
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
		if seenDocCount[chunk.DocID] >= 3 {
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

func recallAtK(chunks []types.RetrievalChunk, expectedDocIDs, expectedChunkIDs []string) (int, int, float64) {
	expectedChunks := normalizeIDSet(expectedChunkIDs)
	expectedDocs := normalizeIDSet(expectedDocIDs)
	expectedCount := len(expectedChunks) + len(expectedDocs)
	if expectedCount == 0 {
		return 0, 0, 0
	}

	hitChunks := make(map[string]struct{})
	hitDocs := make(map[string]struct{})
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
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		set[id] = struct{}{}
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
	if value > 10 {
		return 10
	}
	return value
}
