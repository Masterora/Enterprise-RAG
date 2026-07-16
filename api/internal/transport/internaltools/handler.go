package internaltools

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"enterprise-rag/api/internal/service/knowledge"
	"enterprise-rag/api/internal/service/retrieval"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/zeromicro/go-zero/rest"
)

const maxRequestBytes = 1 << 20

type Handler struct {
	svcCtx *svc.ServiceContext
}

type requestContext struct {
	TenantID  string `json:"tenant_id"`
	UserID    string `json:"user_id"`
	SubjectID string `json:"subject_id"`
}

type searchRequest struct {
	requestContext
	Query            string   `json:"query"`
	SearchQuery      string   `json:"search_query"`
	TopK             int      `json:"top_k"`
	ExpectedDocIDs   []string `json:"expected_doc_ids"`
	ExpectedChunkIDs []string `json:"expected_chunk_ids"`
	ExpectedRoute    string   `json:"expected_route"`
}

type navigationRequest struct {
	requestContext
	Topic string `json:"topic"`
}

type modelCredentialsRequest struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
}

type modelCredentialsResponse struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
}

type toolResponse struct {
	Content  string                 `json:"content"`
	Chunks   []types.RetrievalChunk `json:"chunks"`
	Metrics  types.RetrievalMetrics `json:"metrics"`
	Stages   []string               `json:"stages"`
	Coverage toolCoverage           `json:"coverage"`
}

type toolCoverage struct {
	TotalDocuments   int  `json:"total_documents"`
	CoveredDocuments int  `json:"covered_documents"`
	Complete         bool `json:"complete"`
}

func RegisterRoutes(server *rest.Server, svcCtx *svc.ServiceContext) {
	handler := &Handler{svcCtx: svcCtx}
	server.AddRoutes([]rest.Route{
		{Method: http.MethodPost, Path: "/internal/v1/tools/knowledge-search", Handler: handler.authorize(handler.knowledgeSearch)},
		{Method: http.MethodPost, Path: "/internal/v1/tools/knowledge-overview", Handler: handler.authorize(handler.knowledgeOverview)},
		{Method: http.MethodPost, Path: "/internal/v1/tools/document-navigation", Handler: handler.authorize(handler.documentNavigation)},
		{Method: http.MethodPost, Path: "/internal/v1/tools/model-credentials", Handler: handler.authorize(handler.modelCredentials)},
	})
}

func (h *Handler) modelCredentials(w http.ResponseWriter, r *http.Request) {
	var req modelCredentialsRequest
	if err := decodeRequest(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.UserID = strings.TrimSpace(req.UserID)
	if req.TenantID == "" || req.UserID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id and user_id are required")
		return
	}
	user, err := h.svcCtx.UserRepo.GetByID(r.Context(), req.UserID)
	if err != nil || user.TenantID != req.TenantID {
		writeError(w, http.StatusForbidden, "model credentials are not available for this context")
		return
	}
	apiKey, err := h.svcCtx.ModelSettings.ResolveAPIKey(r.Context(), req.TenantID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "model API key is not configured")
		return
	}
	writeJSON(w, http.StatusOK, modelCredentialsResponse{
		Provider: h.svcCtx.Config.Embedding.Provider,
		BaseURL:  h.svcCtx.Config.Embedding.BaseURL,
		APIKey:   apiKey,
	})
}

func (h *Handler) authorize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provided := []byte(r.Header.Get("X-Service-Token"))
		expected := []byte(h.svcCtx.Config.AgentService.ServiceToken)
		if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (h *Handler) knowledgeSearch(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if err := decodeRequest(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateContext(req.requestContext); err != nil || strings.TrimSpace(req.Query) == "" {
		writeError(w, http.StatusBadRequest, "tenant_id, user_id, subject_id and query are required")
		return
	}
	stages := make([]string, 0, 10)
	chunks, metrics, err := retrieval.NewService(h.svcCtx).SearchWithOptions(
		r.Context(), req.TenantID, req.UserID, req.SubjectID, req.Query,
		retrieval.SearchOptions{
			TopK: req.TopK, SearchQuery: req.SearchQuery,
			ExpectedDocIDs: req.ExpectedDocIDs, ExpectedChunkIDs: req.ExpectedChunkIDs, ExpectedRoute: req.ExpectedRoute,
			OnStage: func(stage string) error {
				stages = append(stages, stage)
				return nil
			},
		},
	)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, newToolResponse("", chunks, metrics, stages))
}

func (h *Handler) knowledgeOverview(w http.ResponseWriter, r *http.Request) {
	var req requestContext
	if err := decodeRequest(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateContext(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := knowledge.BuildKnowledgeOverviewTool(r.Context(), h.svcCtx, req.TenantID, req.UserID, req.SubjectID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response := newToolResponse(
		result.Content,
		result.Chunks,
		overviewMetrics("overview", len(result.Chunks)),
		nil,
	)
	response.Coverage = toolCoverage{
		TotalDocuments:   result.TotalDocuments,
		CoveredDocuments: result.CoveredDocuments,
		Complete: result.TotalDocuments > 0 &&
			result.CoveredDocuments == result.TotalDocuments,
	}
	response.Metrics.CandidateCount = result.TotalDocuments
	response.Metrics.EvaluationPassed = response.Coverage.Complete
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) documentNavigation(w http.ResponseWriter, r *http.Request) {
	var req navigationRequest
	if err := decodeRequest(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateContext(req.requestContext); err != nil || strings.TrimSpace(req.Topic) == "" {
		writeError(w, http.StatusBadRequest, "tenant_id, user_id, subject_id and topic are required")
		return
	}
	content, chunks, err := knowledge.BuildDocumentNavigationTool(r.Context(), h.svcCtx, req.TenantID, req.UserID, req.SubjectID, req.Topic)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, newToolResponse(content, chunks, overviewMetrics("navigation", len(chunks)), nil))
}

func newToolResponse(content string, chunks []types.RetrievalChunk, metrics types.RetrievalMetrics, stages []string) toolResponse {
	if chunks == nil {
		chunks = make([]types.RetrievalChunk, 0)
	}
	if stages == nil {
		stages = make([]string, 0)
	}
	return toolResponse{Content: content, Chunks: chunks, Metrics: metrics, Stages: stages}
}

func decodeRequest(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid request body")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("invalid request body")
	}
	return nil
}

func validateContext(req requestContext) error {
	if strings.TrimSpace(req.TenantID) == "" || strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.SubjectID) == "" {
		return errors.New("tenant_id, user_id and subject_id are required")
	}
	return nil
}

func overviewMetrics(route string, count int) types.RetrievalMetrics {
	return types.RetrievalMetrics{
		Route: route, RouteCorrect: true, ReturnedCount: count, CitationCount: count, EvaluationPassed: count > 0,
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
