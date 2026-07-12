package retrieval

import (
	"testing"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/types"
)

func TestEvaluateRetrievalUsesConfiguredThresholds(t *testing.T) {
	thresholds := config.EvaluationConf{MinRecallAtK: 0.8, MaxLatencyMS: 1000}
	metrics := types.RetrievalMetrics{ExpectedCount: 2, RecallAtK: 0.5, RouteCorrect: true, LatencyMS: 100}
	if evaluateRetrieval(metrics, "rag", thresholds) {
		t.Fatal("expected evaluation to fail below recall threshold")
	}
	metrics.RecallAtK = 1
	if !evaluateRetrieval(metrics, "rag", thresholds) {
		t.Fatal("expected evaluation to pass")
	}
}
