package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunnerAndReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization header = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/api/retrieval/search" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"metrics":{"recall_at_k":1,"route_correct":true,"evaluation_passed":true,"latency_ms":12,"candidate_count":15,"returned_count":3}}`))
	}))
	defer server.Close()

	evaluationRunner, err := newRunner(server.URL, "test-token", 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	evaluationSuite := suite{
		Name: "smoke", Mode: modeRetrieval, SubjectID: "subject", TopK: 5,
		Cases: []testCase{{Name: "case", Query: "question", ExpectedDocIDs: []string{"document"}, ExpectedRoute: "rag"}},
	}
	results := evaluationRunner.run(context.Background(), evaluationSuite)
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v", results)
	}
	report := renderReport(evaluationSuite, results, time.Unix(0, 0))
	for _, expected := range []string{"平均 Recall@K：100.0%", "路由正确率：100.0%", "| case | 通过 | 100.0% | 通过 |"} {
		if !strings.Contains(report, expected) {
			t.Fatalf("report does not contain %q:\n%s", expected, report)
		}
	}
}

func TestSuiteValidation(t *testing.T) {
	evaluationSuite := suite{Mode: "unknown", SubjectID: "subject", Cases: []testCase{{Query: "question"}}}
	if err := evaluationSuite.validate(); err == nil {
		t.Fatal("expected invalid mode error")
	}
}
