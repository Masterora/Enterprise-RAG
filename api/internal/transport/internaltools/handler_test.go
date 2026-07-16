package internaltools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"
)

func TestAuthorizeRequiresExactServiceToken(t *testing.T) {
	handler := &Handler{svcCtx: &svc.ServiceContext{Config: config.Config{
		AgentService: config.AgentServiceConf{ServiceToken: "0123456789abcdef"},
	}}}
	nextCalled := false
	next := handler.authorize(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	unauthorized := httptest.NewRequest(http.MethodPost, "/internal", nil)
	unauthorized.Header.Set("X-Service-Token", "wrong-token")
	unauthorizedResponse := httptest.NewRecorder()
	next(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized || nextCalled {
		t.Fatalf("status=%d next_called=%t", unauthorizedResponse.Code, nextCalled)
	}

	authorized := httptest.NewRequest(http.MethodPost, "/internal", nil)
	authorized.Header.Set("X-Service-Token", "0123456789abcdef")
	authorizedResponse := httptest.NewRecorder()
	next(authorizedResponse, authorized)
	if authorizedResponse.Code != http.StatusNoContent || !nextCalled {
		t.Fatalf("status=%d next_called=%t", authorizedResponse.Code, nextCalled)
	}
}

func TestToolResponseSerializesEmptyCollectionsAsArrays(t *testing.T) {
	response := newToolResponse("content", nil, types.RetrievalMetrics{}, nil)
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	value := string(encoded)
	if !strings.Contains(value, `"chunks":[]`) ||
		!strings.Contains(value, `"stages":[]`) ||
		!strings.Contains(value, `"coverage":{"total_documents":0,"covered_documents":0,"complete":false}`) {
		t.Fatalf("response = %s", value)
	}
}

func TestOverviewMetricsFailsWithoutEvidence(t *testing.T) {
	metrics := overviewMetrics("overview", 0)
	if metrics.EvaluationPassed {
		t.Fatal("empty overview must not pass evaluation")
	}
}

func TestDecodeRequestRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/internal",
		strings.NewReader(`{"tenant_id":"tenant","user_id":"user","subject_id":"subject","unknown":true}`),
	)
	var target requestContext
	if err := decodeRequest(request, &target); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestDecodeRequestAcceptsOneJSONObject(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/internal",
		strings.NewReader(`{"tenant_id":"tenant","user_id":"user","subject_id":"subject"}`),
	)
	var target requestContext
	if err := decodeRequest(request, &target); err != nil {
		t.Fatal(err)
	}
	if target.UserID != "user" || target.SubjectID != "subject" {
		t.Fatalf("decoded request = %+v", target)
	}
}
