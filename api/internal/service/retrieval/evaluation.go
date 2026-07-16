package retrieval

import (
	"strings"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/types"
)

func evaluateRetrieval(metrics types.RetrievalMetrics, expectedRoute string, thresholds config.EvaluationConf) bool {
	if metrics.ExpectedCount > 0 && thresholds.MinRecallAtK > 0 && metrics.RecallAtK < thresholds.MinRecallAtK {
		return false
	}
	if strings.TrimSpace(expectedRoute) != "" && !metrics.RouteCorrect {
		return false
	}
	if thresholds.MaxLatencyMS > 0 && metrics.LatencyMS > thresholds.MaxLatencyMS {
		return false
	}
	return true
}
