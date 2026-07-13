package agent

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/service/chatflow"
	"enterprise-rag/backend/internal/types"

	"github.com/google/uuid"
)

func collectToolResults(results []indexedToolResult) ([]types.RetrievalChunk, []types.ExternalLink, types.RetrievalMetrics) {
	chunks := make([]types.RetrievalChunk, 0)
	links := make([]types.ExternalLink, 0)
	chunkIDs := make(map[string]struct{})
	linkURLs := make(map[string]struct{})
	var metrics types.RetrievalMetrics
	for _, item := range results {
		if item.err != nil {
			continue
		}
		if item.result.Metrics.OriginalQuery != "" {
			metrics = item.result.Metrics
		}
		for _, chunk := range item.result.Chunks {
			key := chunk.ID
			if key == "" {
				key = fmt.Sprintf("%s:%d", chunk.DocID, chunk.ChunkIndex)
			}
			if _, exists := chunkIDs[key]; exists {
				continue
			}
			chunkIDs[key] = struct{}{}
			chunks = append(chunks, chunk)
		}
		for _, link := range item.result.ExternalLinks {
			if _, exists := linkURLs[link.URL]; link.URL == "" || exists {
				continue
			}
			linkURLs[link.URL] = struct{}{}
			links = append(links, link)
		}
	}
	return chunks, links, metrics
}

var observationCitationPattern = regexp.MustCompile(`\[(?:引用|外链)\d+\]`)

func formatObservations(results []indexedToolResult, maxRunes int) string {
	lines := make([]string, 0, len(results))
	for _, item := range results {
		if item.err != nil {
			lines = append(lines, fmt.Sprintf("- %s：执行失败，结果不可用。", item.call.Name))
			continue
		}
		content := observationCitationPattern.ReplaceAllString(item.result.Content, "")
		lines = append(lines, fmt.Sprintf("- %s：%s", item.call.Name, strings.TrimSpace(content)))
	}
	return truncateRunes(strings.Join(lines, "\n"), maxRunes)
}

func formatPlanningObservations(results []indexedToolResult, maxRunes int) string {
	lines := make([]string, 0, len(results))
	for _, item := range results {
		if item.err != nil {
			lines = append(lines, fmt.Sprintf("- %s：执行失败，结果不可用。", item.call.Name))
			continue
		}
		content := strings.TrimSpace(observationCitationPattern.ReplaceAllString(item.result.Content, ""))
		if len(item.result.Chunks) > 0 {
			content += "\n" + formatChunks(item.result.Chunks)
		}
		if len(item.result.ExternalLinks) > 0 {
			content += "\n" + formatLinks(item.result.ExternalLinks)
		}
		lines = append(lines, fmt.Sprintf("- %s：%s", item.call.Name, strings.TrimSpace(content)))
	}
	return truncateRunes(strings.Join(lines, "\n"), maxRunes)
}

func formatChunks(chunks []types.RetrievalChunk) string {
	if len(chunks) == 0 {
		return "无"
	}
	var builder strings.Builder
	for index, chunk := range chunks {
		fmt.Fprintf(&builder, "[引用%d]\n文档：%s\n章节：%s\n页码：%d\n内容：%s\n\n", index+1, chunk.DocName, chunk.Section, chunk.Page, chunk.Content)
	}
	return strings.TrimSpace(builder.String())
}

func formatLinks(links []types.ExternalLink) string {
	if len(links) == 0 {
		return "无"
	}
	var builder strings.Builder
	for index, link := range links {
		fmt.Fprintf(&builder, "[外链%d]\n标题：%s\n链接：%s\n摘要：%s\n\n", index+1, link.Title, link.URL, link.Snippet)
	}
	return strings.TrimSpace(builder.String())
}

func completeMetrics(metrics types.RetrievalMetrics, plans []Plan, input Input, answer string, citationCount int, startedAt time.Time, thresholds config.EvaluationConf) types.RetrievalMetrics {
	if metrics.OriginalQuery == "" {
		metrics.OriginalQuery = input.Question
		metrics.SearchQuery = input.Question
	}
	metrics.Route = routeFromPlans(plans)
	metrics.RouteCorrect = strings.TrimSpace(input.ExpectedRoute) == "" || strings.EqualFold(input.ExpectedRoute, metrics.Route)
	metrics.EvaluationPassed = metrics.RouteCorrect
	return chatflow.CompleteAnswerMetrics(metrics, answer, input.ExpectedOutcome, citationCount, startedAt, thresholds)
}

func routeFromPlans(plans []Plan) string {
	for _, plan := range plans {
		for _, call := range plan.Tools {
			switch call.Name {
			case ToolKnowledgeOverview:
				return "overview"
			case ToolDocumentNavigation:
				return "navigation"
			case ToolKnowledgeSearch:
				return "rag"
			}
		}
	}
	return "direct"
}

func newStep(kind, title, tool string) types.AgentStep {
	return types.AgentStep{ID: uuid.NewString(), Kind: kind, Title: title, Tool: tool, Status: "running"}
}

func truncateRunes(value string, limit int) string {
	if limit < 1 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
