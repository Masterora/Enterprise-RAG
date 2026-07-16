package milvus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/model"
)

type Store struct {
	baseURL string
	client  *http.Client
	config  config.MilvusConf
}

func NewStore(ctx context.Context, c config.MilvusConf) (*Store, error) {
	store := &Store{
		baseURL: normalizeBaseURL(c.Address),
		client:  &http.Client{Timeout: 30 * time.Second},
		config:  c,
	}
	if err := store.ensureCollection(ctx); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() {
	s.client.CloseIdleConnections()
}

func (s *Store) ensureCollection(ctx context.Context) error {
	exists, err := s.hasCollection(ctx)
	if err != nil {
		return err
	}
	if !exists {
		payload := map[string]any{
			"collectionName": s.config.Collection,
			"schema": map[string]any{
				"enableDynamicField": false,
				"fields": []map[string]any{
					{"fieldName": "id", "dataType": "VarChar", "isPrimary": true, "elementTypeParams": map[string]any{"max_length": 64}},
					{"fieldName": "tenant_id", "dataType": "VarChar", "elementTypeParams": map[string]any{"max_length": 64}},
					{"fieldName": "doc_id", "dataType": "VarChar", "elementTypeParams": map[string]any{"max_length": 64}},
					{"fieldName": "subject_id", "dataType": "VarChar", "elementTypeParams": map[string]any{"max_length": 64}},
					{"fieldName": "user_id", "dataType": "VarChar", "elementTypeParams": map[string]any{"max_length": 64}},
					{"fieldName": "chunk_index", "dataType": "Int64"},
					{"fieldName": "page", "dataType": "Int64"},
					{"fieldName": "section", "dataType": "VarChar", "elementTypeParams": map[string]any{"max_length": 512}},
					{"fieldName": "content", "dataType": "VarChar", "elementTypeParams": map[string]any{"max_length": 65535}},
					{"fieldName": "document_version", "dataType": "Int64"},
					{"fieldName": "content_hash", "dataType": "VarChar", "elementTypeParams": map[string]any{"max_length": 64}},
					{"fieldName": "embedding_model", "dataType": "VarChar", "elementTypeParams": map[string]any{"max_length": 256}},
					{"fieldName": "chunk_strategy_version", "dataType": "VarChar", "elementTypeParams": map[string]any{"max_length": 64}},
					{"fieldName": "vector", "dataType": "FloatVector", "elementTypeParams": map[string]any{"dim": s.config.Dimension}},
				},
			},
			"indexParams": []map[string]any{
				{
					"fieldName":  "vector",
					"indexName":  "vector",
					"metricType": "COSINE",
					"params": map[string]any{
						"index_type":     "HNSW",
						"M":              16,
						"efConstruction": 200,
					},
				},
			},
		}
		if err := s.do(ctx, "/v2/vectordb/collections/create", payload); err != nil {
			return err
		}
	}

	return s.do(ctx, "/v2/vectordb/collections/load", map[string]any{
		"collectionName": s.config.Collection,
	})
}

func (s *Store) UpsertChunks(ctx context.Context, chunks []model.DocumentChunk, vectors [][]float32) error {
	entities := make([]map[string]any, 0, len(chunks))
	for index, chunk := range chunks {
		entities = append(entities, map[string]any{
			"id":                     chunk.ID,
			"tenant_id":              chunk.TenantID,
			"doc_id":                 chunk.DocID,
			"subject_id":             chunk.SubjectID,
			"user_id":                chunk.UserID,
			"chunk_index":            chunk.ChunkIndex,
			"page":                   chunk.Page,
			"section":                chunk.Section,
			"content":                chunk.Content,
			"document_version":       chunk.DocumentVersion,
			"content_hash":           chunk.ContentHash,
			"embedding_model":        chunk.EmbeddingModel,
			"chunk_strategy_version": chunk.ChunkStrategyVersion,
			"vector":                 vectors[index],
		})
	}

	return s.do(ctx, "/v2/vectordb/entities/upsert", map[string]any{
		"collectionName": s.config.Collection,
		"data":           entities,
	})
}

func (s *Store) Search(ctx context.Context, tenantID, subjectID, embeddingModel string, vector []float32, topK int) ([]model.RetrievalChunk, error) {
	if topK <= 0 {
		topK = 5
	}

	var result []map[string]any
	if err := s.doWithResponse(ctx, "/v2/vectordb/entities/search", map[string]any{
		"collectionName": s.config.Collection,
		"data":           [][]float32{vector},
		"annsField":      "vector",
		"filter": fmt.Sprintf(
			`tenant_id == "%s" AND subject_id == "%s" AND embedding_model == "%s"`,
			escapeFilterValue(tenantID), escapeFilterValue(subjectID), escapeFilterValue(embeddingModel),
		),
		"limit":        topK,
		"outputFields": []string{"tenant_id", "doc_id", "subject_id", "user_id", "chunk_index", "page", "section", "content", "document_version", "content_hash"},
		"searchParams": map[string]any{
			"metricType": s.config.MetricType,
			"params": map[string]any{
				"ef": 64,
			},
		},
	}, &result); err != nil {
		return nil, err
	}

	chunks := make([]model.RetrievalChunk, 0, len(result))
	for _, item := range result {
		chunks = append(chunks, model.RetrievalChunk{
			ID:              asString(item["id"]),
			EvidenceID:      asString(item["id"]),
			TenantID:        asString(item["tenant_id"]),
			DocID:           asString(item["doc_id"]),
			SubjectID:       asString(item["subject_id"]),
			UserID:          asString(item["user_id"]),
			ChunkIndex:      asInt64(item["chunk_index"]),
			Page:            asInt64(item["page"]),
			Section:         asString(item["section"]),
			Content:         asString(item["content"]),
			DocumentVersion: int(asInt64(item["document_version"])),
			ContentHash:     asString(item["content_hash"]),
			Score:           asFloat64(item["distance"]),
		})
	}
	return chunks, nil
}

func (s *Store) DeleteByDocIDs(ctx context.Context, userID string, docIDs []string) error {
	if len(docIDs) == 0 {
		return nil
	}

	quoted := make([]string, 0, len(docIDs))
	for _, docID := range docIDs {
		docID = strings.TrimSpace(docID)
		if docID == "" {
			continue
		}
		quoted = append(quoted, fmt.Sprintf(`"%s"`, escapeFilterValue(docID)))
	}
	if len(quoted) == 0 {
		return nil
	}

	filter := fmt.Sprintf(`user_id == "%s" AND doc_id in [%s]`, escapeFilterValue(userID), strings.Join(quoted, ", "))
	return s.deleteByFilter(ctx, filter)
}

func (s *Store) HasDocVectors(ctx context.Context, userID, docID string) (bool, error) {
	docID = strings.TrimSpace(docID)
	userID = strings.TrimSpace(userID)
	if docID == "" || userID == "" {
		return false, nil
	}

	var result []map[string]any
	err := s.doWithResponse(ctx, "/v2/vectordb/entities/query", map[string]any{
		"collectionName": s.config.Collection,
		"filter":         fmt.Sprintf(`user_id == "%s" AND doc_id == "%s"`, escapeFilterValue(userID), escapeFilterValue(docID)),
		"limit":          1,
		"outputFields":   []string{"id"},
	}, &result)
	if err != nil {
		return false, err
	}
	return len(result) > 0, nil
}

func (s *Store) DeleteBySubject(ctx context.Context, userID, subjectID string) error {
	subjectID = strings.TrimSpace(subjectID)
	if subjectID == "" {
		return nil
	}

	filter := fmt.Sprintf(`user_id == "%s" AND subject_id == "%s"`, escapeFilterValue(userID), escapeFilterValue(subjectID))
	return s.deleteByFilter(ctx, filter)
}

func (s *Store) Ready(ctx context.Context) error {
	exists, err := s.hasCollection(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("collection %q does not exist", s.config.Collection)
	}
	return nil
}

func (s *Store) hasCollection(ctx context.Context) (bool, error) {
	var collections []string
	if err := s.doWithResponse(ctx, "/v2/vectordb/collections/list", map[string]any{}, &collections); err != nil {
		return false, err
	}
	for _, name := range collections {
		if name == s.config.Collection {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) do(ctx context.Context, path string, payload any) error {
	return s.doWithResponse(ctx, path, payload, nil)
}

func (s *Store) doWithResponse(ctx context.Context, path string, payload any, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var parsed struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	if parsed.Code != 0 {
		return errors.New(parsed.Message)
	}
	if result != nil && len(parsed.Data) > 0 {
		if err := json.Unmarshal(parsed.Data, result); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) deleteByFilter(ctx context.Context, filter string) error {
	return s.do(ctx, "/v2/vectordb/entities/delete", map[string]any{
		"collectionName": s.config.Collection,
		"filter":         filter,
	})
}

func normalizeBaseURL(address string) string {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return address
	}
	return "http://" + address
}

func escapeFilterValue(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		return ""
	}
}

func asInt64(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	default:
		return 0
	}
}

func asFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	default:
		return 0
	}
}
