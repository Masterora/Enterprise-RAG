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
)

type routeDecision struct {
	route          QueryRoute
	score          int
	competingScore int
}

func RouteQuery(query string) QueryRoute {
	return decideRoute(query).route
}

func ResolveRoutedAnswer(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, subjectID, query, llmProvider, llmModel string,
) (QueryRoute, string, []types.RetrievalChunk, bool, error) {
	decision := decideRoute(query)
	logx.WithContext(ctx).Infof("chat route decision: route=%s score=%d competing=%d query=%q", decision.route, decision.score, decision.competingScore, query)

	switch decision.route {
	case QueryRouteOverview:
		answer, chunks, err := buildKnowledgeOverview(ctx, svcCtx, userID, subjectID, query, llmProvider, llmModel)
		return QueryRouteOverview, answer, chunks, true, err
	case QueryRouteNavigation:
		answer, chunks, err := buildDocumentNavigation(ctx, svcCtx, userID, subjectID, query)
		return QueryRouteNavigation, answer, chunks, true, err
	default:
		return QueryRouteRAG, "", nil, false, nil
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
