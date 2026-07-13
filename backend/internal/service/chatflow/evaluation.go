package chatflow

import (
	"strings"
	"time"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/types"
)

func CompleteAnswerMetrics(
	metrics types.RetrievalMetrics,
	answer, expectedOutcome string,
	citationCount int,
	startedAt time.Time,
	thresholds config.EvaluationConf,
) types.RetrievalMetrics {
	metrics.LatencyMS = time.Since(startedAt).Milliseconds()
	metrics.Answered = strings.TrimSpace(answer) != "" && !IsNoAnswer(answer)
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

func IsNoAnswer(answer string) bool {
	normalized := strings.Trim(strings.TrimSpace(answer), "。.!！ \n\t")
	switch strings.ToLower(normalized) {
	case "无法确定", "unable to determine", "cannot determine", "判断できません":
		return true
	default:
		return false
	}
}
