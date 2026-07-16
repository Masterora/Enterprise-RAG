package observability

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsExposeOperationalSignals(t *testing.T) {
	metrics := NewMetrics()
	metrics.AgentStarted()
	metrics.AgentFinished()
	metrics.WorkerStarted("document.parse")
	metrics.WorkerFinished("document.parse")
	metrics.ObserveAgentRun("success", true, time.Second, 2)
	metrics.ObserveAgentTransition("planning", "executing")
	metrics.ObserveTool("knowledge_search", "success", 100*time.Millisecond)
	metrics.ObserveModel("llm", "agent_plan", "openrouter", "success", time.Second)
	metrics.ObserveModelUsage("llm", "agent_plan", "openrouter", ModelUsage{
		InputTokens: 100, OutputTokens: 20, TotalTokens: 120, CostUSD: 0.0015,
	})
	metrics.ObserveRetrieval("success", 200*time.Millisecond, 15, 3)
	metrics.ObserveWorker("document.parse", "retry", time.Second)

	server := httptest.NewServer(metrics.Handler())
	defer server.Close()
	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"enterprise_rag_agent_runs_total", "enterprise_rag_agent_state_transitions_total",
		"enterprise_rag_agent_tool_calls_total", "enterprise_rag_model_calls_total",
		"enterprise_rag_model_tokens_total", "enterprise_rag_model_cost_usd_total",
		"enterprise_rag_retrieval_runs_total", "enterprise_rag_worker_tasks_total",
		"enterprise_rag_agent_runs_in_flight", "enterprise_rag_worker_tasks_in_flight",
	} {
		if !strings.Contains(string(body), name) {
			t.Fatalf("metrics output does not contain %s", name)
		}
	}
	if !strings.Contains(string(body), `enterprise_rag_worker_tasks_total{status="retry",task_type="document_parse"} 1`) {
		t.Fatalf("worker task type was not normalized: %s", body)
	}
	if !strings.Contains(string(body), `enterprise_rag_model_tokens_total{kind="llm",operation="agent_plan",provider="openrouter",token_type="total"} 120`) {
		t.Fatalf("model token usage was not recorded: %s", body)
	}
	if !strings.Contains(string(body), `enterprise_rag_model_cost_usd_total{kind="llm",operation="agent_plan",provider="openrouter"} 0.0015`) {
		t.Fatalf("model cost was not recorded: %s", body)
	}
}

func TestHTTPMiddlewareRecordsNormalizedRouteAndStatus(t *testing.T) {
	metrics := NewMetrics()
	handler := metrics.HTTPMiddleware("/metrics")(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodPost, "/api/chat/ask", nil))

	metricsRecorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricsRecorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := metricsRecorder.Body.String()
	if !strings.Contains(body, `enterprise_rag_http_requests_total{method="POST",route="/api/chat/ask",status_class="2xx"} 1`) {
		t.Fatalf("unexpected metrics output: %s", body)
	}
}
