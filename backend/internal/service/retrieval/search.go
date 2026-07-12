package retrieval

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

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
	ExpectedRoute    string
	LLMProvider      string
	LLMModel         string
	SearchQuery      string
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
	startedAt := time.Now()
	subjectID = strings.TrimSpace(subjectID)
	query = strings.TrimSpace(query)
	if subjectID == "" || query == "" {
		return nil, types.RetrievalMetrics{}, errors.New("subject_id and query are required")
	}
	retrievalConf := s.svcCtx.Config.Retrieval
	finalTopK := normalizeTopK(options.TopK, retrievalConf.TopK)
	candidateTopK := finalTopK
	if s.svcCtx.Config.Retrieval.Rerank {
		candidateTopK *= normalizeMultiplier(retrievalConf.CandidateMultiplier)
	}
	candidateTopK = min(candidateTopK, normalizeCandidateLimit(retrievalConf.CandidateLimit))

	metrics := types.RetrievalMetrics{
		OriginalQuery: query,
		SearchQuery:   query,
		TopK:          finalTopK,
		Reranked:      s.svcCtx.Config.Retrieval.Rerank,
		Route:         "rag",
		RouteCorrect:  strings.TrimSpace(options.ExpectedRoute) == "" || strings.EqualFold(strings.TrimSpace(options.ExpectedRoute), "rag"),
	}

	exists, err := s.svcCtx.SubjectRepo.ExistsAccessible(ctx, subjectID, userID)
	if err != nil {
		return nil, metrics, err
	}
	if !exists {
		return nil, metrics, errors.New("knowledge base not found")
	}

	searchQuery := strings.TrimSpace(options.SearchQuery)
	if searchQuery != "" {
		metrics.SearchQuery = searchQuery
		metrics.QueryRewritten = !strings.EqualFold(searchQuery, query)
		if err := notifyStage(options.OnStage, "retrieval.rewrite.done"); err != nil {
			return nil, metrics, err
		}
	} else if s.svcCtx.Config.Retrieval.QueryRewrite {
		searchQuery = query
		if err := notifyStage(options.OnStage, "retrieval.rewrite.start"); err != nil {
			return nil, metrics, err
		}
		rewritten, err := s.rewriteQuery(ctx, query, options.LLMProvider, options.LLMModel)
		if err != nil {
			logx.WithContext(ctx).Errorf("query rewrite failed, fallback to original query: subject_id=%s err=%v", subjectID, err)
			if err := notifyStage(options.OnStage, "retrieval.rewrite.fallback"); err != nil {
				return nil, metrics, err
			}
		} else if rewritten != "" && rewritten != query {
			searchQuery = rewritten
			metrics.SearchQuery = rewritten
			metrics.QueryRewritten = true
			if err := notifyStage(options.OnStage, "retrieval.rewrite.done"); err != nil {
				return nil, metrics, err
			}
		} else {
			if err := notifyStage(options.OnStage, "retrieval.rewrite.skipped"); err != nil {
				return nil, metrics, err
			}
		}
	} else {
		searchQuery = query
		if err := notifyStage(options.OnStage, "retrieval.rewrite.disabled"); err != nil {
			return nil, metrics, err
		}
	}

	plan := buildQueryPlan(query, searchQuery, retrievalConf.MaxQueryRunes, retrievalConf.MaxSubQueries)
	metrics.SubQueryCount = len(plan.queries)
	if len(plan.queries) > 1 {
		if err := notifyStage(options.OnStage, "retrieval.query.split"); err != nil {
			return nil, metrics, err
		}
	}
	if err := notifyStage(options.OnStage, "retrieval.embedding.start"); err != nil {
		return nil, metrics, err
	}
	vectors, err := s.svcCtx.Embedder.Embed(ctx, plan.queries)
	if err != nil {
		return nil, metrics, err
	}
	if len(vectors) == 0 {
		return nil, metrics, errors.New("query embedding is empty")
	}

	if err := notifyStage(options.OnStage, "retrieval.vector.start"); err != nil {
		return nil, metrics, err
	}
	vectorMatches := make(map[string]model.RetrievalChunk)
	for _, vector := range vectors {
		chunks, err := s.svcCtx.MilvusStore.Search(ctx, subjectID, vector, candidateTopK)
		if err != nil {
			return nil, metrics, err
		}
		for _, chunk := range chunks {
			if existing, ok := vectorMatches[chunk.ID]; !ok || chunk.Score > existing.Score {
				vectorMatches[chunk.ID] = chunk
			}
		}
	}

	vectorChunks := make([]types.RetrievalChunk, 0, len(vectorMatches))
	for _, chunk := range vectorMatches {
		if retrievalConf.SimilarityThreshold > 0 && chunk.Score < retrievalConf.SimilarityThreshold {
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

	if err := notifyStage(options.OnStage, "retrieval.keyword.start"); err != nil {
		return nil, metrics, err
	}
	keywords, err := s.keywordSearch(ctx, subjectID, strings.Join(plan.queries, " "), candidateTopK)
	if err != nil {
		return nil, metrics, err
	}

	if err := notifyStage(options.OnStage, "retrieval.merge.start"); err != nil {
		return nil, metrics, err
	}
	normalizeScores(vectorChunks, keywords)
	merged := mergeChunks(keywords, vectorChunks)
	merged = limitCandidates(merged, candidateTopK)
	metrics.CandidateCount = len(merged)
	if s.svcCtx.Config.Retrieval.Rerank {
		if err := notifyStage(options.OnStage, "retrieval.rerank.start"); err != nil {
			return nil, metrics, err
		}
		rerankChunks(query, merged)
	} else if err := notifyStage(options.OnStage, "retrieval.rerank.skipped"); err != nil {
		return nil, metrics, err
	}
	if err := notifyStage(options.OnStage, "retrieval.citations.trim"); err != nil {
		return nil, metrics, err
	}
	filtered := trimEffectiveChunks(merged, citationPolicy{
		limit:                min(finalTopK, normalizeMaxCitations(retrievalConf.MaxCitations)),
		absoluteThreshold:    retrievalConf.SimilarityThreshold,
		relativeThreshold:    retrievalConf.RelativeScoreThreshold,
		maxChunksPerDocument: retrievalConf.MaxChunksPerDocument,
	})
	metrics.ReturnedCount = len(filtered)
	metrics.LatencyMS = time.Since(startedAt).Milliseconds()
	metrics.ExpectedCount, metrics.RecallHitCount, metrics.RecallAtK = recallAtK(filtered, options.ExpectedDocIDs, options.ExpectedChunkIDs)
	metrics.EvaluationPassed = evaluateRetrieval(metrics, options.ExpectedRoute, s.svcCtx.Config.Evaluation)
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
