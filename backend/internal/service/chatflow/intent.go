package chatflow

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryAnalysis struct {
	Route       QueryRoute
	SearchQuery string
}

func AnalyzeQuery(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	query, llmProvider, llmModel string,
) QueryAnalysis {
	if strings.TrimSpace(query) == "" {
		return QueryAnalysis{Route: QueryRouteFallback}
	}

	client, err := ResolveLLM(ctx, svcCtx, &types.ChatAskReq{LlmProvider: llmProvider, LlmModel: llmModel})
	if err != nil {
		logx.WithContext(ctx).Errorf("chat route llm init failed, fallback to rag: err=%v", err)
		return QueryAnalysis{Route: QueryRouteRAG}
	}

	timeout := time.Duration(svcCtx.Config.Retrieval.RewriteTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	routeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, err := client.Generate(routeCtx, BuildRoutePrompt(svcCtx.Config.Prompt, query), false)
	if err != nil {
		logx.WithContext(ctx).Errorf("chat route classification failed, fallback to rag: err=%v", err)
		return QueryAnalysis{Route: QueryRouteRAG}
	}
	analysis, ok := parseRouteResponse(response)
	if !ok {
		logx.WithContext(ctx).Errorf("chat route returned invalid response, fallback to rag: response=%q", response)
		return QueryAnalysis{Route: QueryRouteRAG}
	}
	if analysis.Route == QueryRouteRAG || analysis.Route == QueryRouteNavigation {
		analysis.SearchQuery = sanitizeRouteSearchQuery(query, analysis.SearchQuery)
	}
	return analysis
}

func parseRouteResponse(response string) (QueryAnalysis, bool) {
	response = strings.TrimSpace(response)
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start < 0 || end < start {
		return QueryAnalysis{}, false
	}
	var result struct {
		Route       string `json:"route"`
		SearchQuery string `json:"search_query"`
	}
	if err := json.Unmarshal([]byte(response[start:end+1]), &result); err != nil {
		return QueryAnalysis{}, false
	}
	route := QueryRoute(strings.ToLower(strings.TrimSpace(result.Route)))
	switch route {
	case QueryRouteRAG, QueryRouteOverview, QueryRouteNavigation, QueryRouteFallback:
		return QueryAnalysis{Route: route, SearchQuery: strings.TrimSpace(result.SearchQuery)}, true
	default:
		return QueryAnalysis{}, false
	}
}

func sanitizeRouteSearchQuery(original, searchQuery string) string {
	searchQuery = strings.Trim(strings.TrimSpace(searchQuery), "`\"'“”‘’")
	searchQuery = strings.Join(strings.Fields(strings.ReplaceAll(searchQuery, "\n", " ")), " ")
	if searchQuery == "" || strings.EqualFold(searchQuery, "无法确定") {
		return strings.TrimSpace(original)
	}
	if runes := []rune(searchQuery); len(runes) > 120 {
		return string(runes[:120])
	}
	return searchQuery
}
