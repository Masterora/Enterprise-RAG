package observability

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zeromicro/go-zero/rest"
)

const namespace = "enterprise_rag"

type Metrics struct {
	registry            *prometheus.Registry
	httpRequests        *prometheus.CounterVec
	httpDuration        *prometheus.HistogramVec
	httpInFlight        *prometheus.GaugeVec
	agentRuns           *prometheus.CounterVec
	agentDuration       *prometheus.HistogramVec
	agentInFlight       prometheus.Gauge
	agentTransitions    *prometheus.CounterVec
	agentIterations     prometheus.Histogram
	toolCalls           *prometheus.CounterVec
	toolDuration        *prometheus.HistogramVec
	modelCalls          *prometheus.CounterVec
	modelDuration       *prometheus.HistogramVec
	modelTokens         *prometheus.CounterVec
	modelCost           *prometheus.CounterVec
	retrievalRuns       *prometheus.CounterVec
	retrievalDuration   *prometheus.HistogramVec
	retrievalCandidates prometheus.Histogram
	retrievalReturned   prometheus.Histogram
	workerTasks         *prometheus.CounterVec
	workerDuration      *prometheus.HistogramVec
	workerInFlight      *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	m := &Metrics{
		registry: registry,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "http_requests_total", Help: "Total HTTP requests by normalized route and status class.",
		}, []string{"method", "route", "status_class"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "http_request_duration_seconds", Help: "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		httpInFlight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Name: "http_requests_in_flight", Help: "Current HTTP requests by normalized route.",
		}, []string{"method", "route"}),
		agentRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "agent_runs_total", Help: "Total Agent runs by outcome and response mode.",
		}, []string{"status", "stream"}),
		agentDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "agent_run_duration_seconds", Help: "Agent run latency in seconds.",
			Buckets: []float64{0.25, 0.5, 1, 2, 5, 10, 20, 45, 90, 180},
		}, []string{"status", "stream"}),
		agentInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Name: "agent_runs_in_flight", Help: "Current Agent runs.",
		}),
		agentTransitions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "agent_state_transitions_total", Help: "Agent state machine transitions.",
		}, []string{"from", "to"}),
		agentIterations: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace, Name: "agent_iterations", Help: "Iterations used by completed and failed Agent runs.",
			Buckets: []float64{1, 2, 3, 4, 5, 6},
		}),
		toolCalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "agent_tool_calls_total", Help: "Agent tool calls by tool and outcome.",
		}, []string{"tool", "status"}),
		toolDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "agent_tool_duration_seconds", Help: "Agent tool latency in seconds.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 25},
		}, []string{"tool", "status"}),
		modelCalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "model_calls_total", Help: "LLM and embedding calls by operation, provider and outcome.",
		}, []string{"kind", "operation", "provider", "status"}),
		modelDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "model_call_duration_seconds", Help: "LLM and embedding call latency in seconds.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 45, 90},
		}, []string{"kind", "operation", "provider", "status"}),
		modelTokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "model_tokens_total", Help: "Model tokens consumed by operation, provider and token type.",
		}, []string{"kind", "operation", "provider", "token_type"}),
		modelCost: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "model_cost_usd_total", Help: "Model cost reported by the provider in US dollars.",
		}, []string{"kind", "operation", "provider"}),
		retrievalRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "retrieval_runs_total", Help: "Retrieval runs by outcome.",
		}, []string{"status"}),
		retrievalDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "retrieval_duration_seconds", Help: "Retrieval pipeline latency in seconds.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20},
		}, []string{"status"}),
		retrievalCandidates: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace, Name: "retrieval_candidates", Help: "Candidate chunks produced before citation trimming.",
			Buckets: []float64{0, 1, 2, 5, 10, 15, 25, 50, 100},
		}),
		retrievalReturned: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace, Name: "retrieval_returned", Help: "Chunks returned after filtering and citation trimming.",
			Buckets: []float64{0, 1, 2, 3, 5, 8, 10, 15},
		}),
		workerTasks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "worker_tasks_total", Help: "Document worker tasks by type and outcome.",
		}, []string{"task_type", "status"}),
		workerDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "worker_task_duration_seconds", Help: "Document worker task latency in seconds.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 180},
		}, []string{"task_type", "status"}),
		workerInFlight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Name: "worker_tasks_in_flight", Help: "Current document worker tasks by type.",
		}, []string{"task_type"}),
	}
	registry.MustRegister(
		prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		m.httpRequests, m.httpDuration, m.httpInFlight, m.agentRuns, m.agentDuration, m.agentInFlight, m.agentTransitions, m.agentIterations,
		m.toolCalls, m.toolDuration, m.modelCalls, m.modelDuration, m.modelTokens, m.modelCost, m.retrievalRuns, m.retrievalDuration,
		m.retrievalCandidates, m.retrievalReturned, m.workerTasks, m.workerDuration, m.workerInFlight,
	)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) HTTPMiddleware(metricsPath string) rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == metricsPath {
				next(w, r)
				return
			}
			startedAt := time.Now()
			route := normalizeRoute(r.URL.Path)
			m.httpInFlight.WithLabelValues(r.Method, route).Inc()
			writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			defer func() {
				m.httpInFlight.WithLabelValues(r.Method, route).Dec()
				m.httpRequests.WithLabelValues(r.Method, route, statusClass(writer.status)).Inc()
				m.httpDuration.WithLabelValues(r.Method, route).Observe(time.Since(startedAt).Seconds())
			}()
			next(writer, r)
		}
	}
}

func (m *Metrics) AgentStarted() {
	if m != nil {
		m.agentInFlight.Inc()
	}
}

func (m *Metrics) AgentFinished() {
	if m != nil {
		m.agentInFlight.Dec()
	}
}

func (m *Metrics) ObserveAgentRun(status string, stream bool, duration time.Duration, iterations int) {
	if m == nil {
		return
	}
	streamValue := strconv.FormatBool(stream)
	status = normalizeStatus(status)
	m.agentRuns.WithLabelValues(status, streamValue).Inc()
	m.agentDuration.WithLabelValues(status, streamValue).Observe(duration.Seconds())
	m.agentIterations.Observe(float64(iterations))
}

func (m *Metrics) ObserveAgentTransition(from, to string) {
	if m != nil {
		m.agentTransitions.WithLabelValues(normalizeLabel(from), normalizeLabel(to)).Inc()
	}
}

func (m *Metrics) ObserveTool(tool, status string, duration time.Duration) {
	if m == nil {
		return
	}
	tool = normalizeLabel(tool)
	status = normalizeStatus(status)
	m.toolCalls.WithLabelValues(tool, status).Inc()
	m.toolDuration.WithLabelValues(tool, status).Observe(duration.Seconds())
}

func (m *Metrics) ObserveModel(kind, operation, provider, status string, duration time.Duration) {
	if m == nil {
		return
	}
	labels := []string{normalizeLabel(kind), normalizeLabel(operation), normalizeLabel(provider), normalizeStatus(status)}
	m.modelCalls.WithLabelValues(labels...).Inc()
	m.modelDuration.WithLabelValues(labels...).Observe(duration.Seconds())
}

func (m *Metrics) ModelUsageContext(ctx context.Context, kind, operation, provider string) context.Context {
	if m == nil {
		return ctx
	}
	return WithModelUsageObserver(ctx, func(usage ModelUsage) {
		m.ObserveModelUsage(kind, operation, provider, usage)
	})
}

func (m *Metrics) ObserveModelUsage(kind, operation, provider string, usage ModelUsage) {
	if m == nil {
		return
	}
	labels := []string{normalizeLabel(kind), normalizeLabel(operation), normalizeLabel(provider)}
	inputTokens := max(usage.InputTokens, 0)
	outputTokens := max(usage.OutputTokens, 0)
	totalTokens := max(usage.TotalTokens, inputTokens+outputTokens)
	if inputTokens > 0 {
		m.modelTokens.WithLabelValues(append(labels, "input")...).Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		m.modelTokens.WithLabelValues(append(labels, "output")...).Add(float64(outputTokens))
	}
	if totalTokens > 0 {
		m.modelTokens.WithLabelValues(append(labels, "total")...).Add(float64(totalTokens))
	}
	if usage.CostUSD > 0 {
		m.modelCost.WithLabelValues(labels...).Add(usage.CostUSD)
	}
}

func (m *Metrics) ObserveRetrieval(status string, duration time.Duration, candidates, returned int) {
	if m == nil {
		return
	}
	status = normalizeStatus(status)
	m.retrievalRuns.WithLabelValues(status).Inc()
	m.retrievalDuration.WithLabelValues(status).Observe(duration.Seconds())
	m.retrievalCandidates.Observe(float64(candidates))
	m.retrievalReturned.Observe(float64(returned))
}

func (m *Metrics) ObserveWorker(taskType, status string, duration time.Duration) {
	if m == nil {
		return
	}
	taskType = normalizeTaskType(taskType)
	status = normalizeStatus(status)
	m.workerTasks.WithLabelValues(taskType, status).Inc()
	m.workerDuration.WithLabelValues(taskType, status).Observe(duration.Seconds())
}

func (m *Metrics) WorkerStarted(taskType string) {
	if m != nil {
		m.workerInFlight.WithLabelValues(normalizeTaskType(taskType)).Inc()
	}
}

func (m *Metrics) WorkerFinished(taskType string) {
	if m != nil {
		m.workerInFlight.WithLabelValues(normalizeTaskType(taskType)).Dec()
	}
}

func normalizeTaskType(value string) string {
	return normalizeLabel(strings.ReplaceAll(value, ".", "_"))
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "success", "completed":
		return "success"
	case "timeout", "deadline_exceeded":
		return "timeout"
	case "retry", "scheduled_retry":
		return "retry"
	default:
		return "error"
	}
}

func normalizeLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 64 {
		return "unknown"
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return "other"
		}
	}
	return value
}

func normalizeRoute(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if len(path) > 96 || !strings.HasPrefix(path, "/api/") {
		return "other"
	}
	return path
}

func statusClass(status int) string {
	if status < 100 || status > 599 {
		return "unknown"
	}
	return fmt.Sprintf("%dxx", status/100)
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *statusWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (w *statusWriter) ReadFrom(reader io.Reader) (int64, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(reader)
	}
	return io.Copy(w.ResponseWriter, reader)
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
