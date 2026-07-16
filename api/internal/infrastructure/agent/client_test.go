package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-rag/api/internal/config"
)

func TestStreamDispatchesEventsAndResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Service-Token") != "0123456789abcdef" {
			t.Fatal("service token was not forwarded")
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"sequence\":1,\"type\":\"delta\",\"payload\":{\"content\":\"回答\"}}\n"))
		_, _ = w.Write([]byte("{\"sequence\":2,\"type\":\"result\",\"payload\":{\"answer\":\"回答\",\"chunks\":[],\"external_links\":[],\"metrics\":{},\"agent_steps\":[]}}\n"))
	}))
	defer server.Close()

	client := NewClient(config.AgentServiceConf{URL: server.URL, ServiceToken: "0123456789abcdef", TimeoutSeconds: 2})
	defer client.Close()
	var delta string
	result, err := client.Stream(context.Background(), Request{RunID: "run"}, Callbacks{
		OnDelta: func(value string) error {
			delta += value
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if delta != "回答" || result.Answer != "回答" {
		t.Fatalf("delta=%q answer=%q", delta, result.Answer)
	}
}

func TestInvokeDecodesResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v1/runs/invoke" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answer":"完成","chunks":[],"external_links":[],"metrics":{},"agent_steps":[]}`))
	}))
	defer server.Close()

	client := NewClient(config.AgentServiceConf{URL: server.URL, ServiceToken: "0123456789abcdef", TimeoutSeconds: 2})
	defer client.Close()
	result, err := client.Invoke(context.Background(), Request{RunID: "run"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Answer != "完成" {
		t.Fatalf("answer = %q, want 完成", result.Answer)
	}
}

func TestReadyChecksAgentReadinessEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/ready" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(config.AgentServiceConf{URL: server.URL, TimeoutSeconds: 2})
	defer client.Close()
	if err := client.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestInvokeRejectsUnknownResponseField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"answer":"完成","unexpected":true}`))
	}))
	defer server.Close()

	client := NewClient(config.AgentServiceConf{URL: server.URL, TimeoutSeconds: 2})
	defer client.Close()
	if _, err := client.Invoke(context.Background(), Request{}); err == nil {
		t.Fatal("expected unknown response field to fail")
	}
}

func TestStreamReturnsCallbackError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"sequence\":1,\"type\":\"status\",\"payload\":{\"message\":\"started\"}}\n"))
	}))
	defer server.Close()

	client := NewClient(config.AgentServiceConf{URL: server.URL, ServiceToken: "0123456789abcdef", TimeoutSeconds: 2})
	defer client.Close()
	want := errors.New("client disconnected")
	_, err := client.Stream(context.Background(), Request{RunID: "run"}, Callbacks{
		OnStatus: func(string) error { return want },
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestStreamRejectsMissingResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"sequence\":1,\"type\":\"delta\",\"payload\":{\"content\":\"partial\"}}\n"))
	}))
	defer server.Close()

	client := NewClient(config.AgentServiceConf{URL: server.URL, ServiceToken: "0123456789abcdef", TimeoutSeconds: 2})
	defer client.Close()
	if _, err := client.Stream(context.Background(), Request{RunID: "run"}, Callbacks{}); err == nil {
		t.Fatal("expected missing result error")
	}
}
