package chatflow

import (
	"context"
	"strings"

	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryRoute string

const (
	QueryRouteRAG        QueryRoute = "rag"
	QueryRouteOverview   QueryRoute = "overview"
	QueryRouteNavigation QueryRoute = "navigation"
	QueryRouteFallback   QueryRoute = "fallback"
)

func ResolveRoutedAnswer(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, subjectID, query, llmProvider, llmModel string,
) (QueryRoute, string, string, []types.RetrievalChunk, bool, error) {
	analysis := AnalyzeQuery(ctx, svcCtx, query, llmProvider, llmModel)
	route := analysis.Route
	logx.WithContext(ctx).Infof("chat route decision: route=%s query=%q", route, query)

	switch route {
	case QueryRouteOverview:
		answer, chunks, err := buildKnowledgeOverview(ctx, svcCtx, userID, subjectID, query, llmProvider, llmModel)
		return QueryRouteOverview, analysis.SearchQuery, answer, chunks, true, err
	case QueryRouteNavigation:
		navigationQuery := analysis.SearchQuery
		if strings.TrimSpace(navigationQuery) == "" {
			navigationQuery = query
		}
		answer, chunks, err := buildDocumentNavigation(ctx, svcCtx, userID, subjectID, navigationQuery)
		return QueryRouteNavigation, analysis.SearchQuery, answer, chunks, true, err
	case QueryRouteFallback:
		return QueryRouteFallback, analysis.SearchQuery, "请提出与当前知识库内容相关的具体问题。", nil, true, nil
	default:
		return QueryRouteRAG, analysis.SearchQuery, "", nil, false, nil
	}
}

func containsAny(text string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func isWeakSection(section string) bool {
	section = strings.TrimSpace(strings.ToLower(section))
	return section == "" || section == "text" || section == "page" || strings.Contains(section, "目录") || strings.Contains(section, "测试问题")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
