package chatflow

import (
	"strings"
	"time"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/types"
)

func RouteMetrics(route QueryRoute, expectedRoute string, startedAt time.Time, thresholds config.EvaluationConf) types.RetrievalMetrics {
	expectedRoute = strings.TrimSpace(expectedRoute)
	metrics := types.RetrievalMetrics{
		Route:        string(route),
		RouteCorrect: expectedRoute == "" || strings.EqualFold(expectedRoute, string(route)),
		LatencyMS:    time.Since(startedAt).Milliseconds(),
	}
	metrics.EvaluationPassed = (expectedRoute == "" || metrics.RouteCorrect) &&
		(thresholds.MaxLatencyMS <= 0 || metrics.LatencyMS <= thresholds.MaxLatencyMS)
	return metrics
}

func CompleteAnswerMetrics(
	metrics types.RetrievalMetrics,
	answer, expectedOutcome string,
	citationCount int,
	startedAt time.Time,
	thresholds config.EvaluationConf,
) types.RetrievalMetrics {
	metrics.LatencyMS = time.Since(startedAt).Milliseconds()
	metrics.Answered = strings.TrimSpace(answer) != "" && strings.TrimSpace(answer) != "无法确定"
	metrics.CitationCount = citationCount
	switch strings.ToLower(strings.TrimSpace(expectedOutcome)) {
	case "answer":
		metrics.OutcomeCorrect = metrics.Answered
	case "no_answer":
		metrics.OutcomeCorrect = !metrics.Answered
	default:
		metrics.OutcomeCorrect = true
	}
	metrics.EvaluationPassed = metrics.EvaluationPassed && metrics.OutcomeCorrect
	if thresholds.MaxLatencyMS > 0 && metrics.LatencyMS > thresholds.MaxLatencyMS {
		metrics.EvaluationPassed = false
	}
	return metrics
}
