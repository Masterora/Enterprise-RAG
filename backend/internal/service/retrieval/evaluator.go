package retrieval

import (
	"context"
	"errors"
	"strings"
	"time"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/service/chatflow"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"
)

type Evaluator struct {
	svcCtx *svc.ServiceContext
}

func NewEvaluator(svcCtx *svc.ServiceContext) *Evaluator {
	return &Evaluator{svcCtx: svcCtx}
}

func (e *Evaluator) Run(ctx context.Context, userID, subjectID string) (*types.RetrievalEvaluateResp, error) {
	exists, err := e.svcCtx.SubjectRepo.ExistsAccessible(ctx, subjectID, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("knowledge base not found")
	}
	cases, err := config.LoadEvaluationCases(e.svcCtx.Config.Evaluation.CasesFile)
	if err != nil {
		return nil, err
	}
	documents, _, err := e.svcCtx.DocumentRepo.ListByUser(ctx, model.DocumentListFilter{
		UserID: userID, SubjectID: subjectID, Status: model.DocumentStatusIndexed, PageSize: 1000,
	})
	if err != nil {
		return nil, err
	}
	documentIDs := make(map[string]string, len(documents))
	for _, document := range documents {
		documentIDs[document.Filename] = document.ID
	}

	response := &types.RetrievalEvaluateResp{Total: len(cases), Cases: make([]types.RetrievalEvaluationCaseResult, 0, len(cases))}
	var recallSum float64
	var recallCases int
	var routeHits int
	var latencySum int64
	for _, testCase := range cases {
		result := e.evaluateCase(ctx, userID, subjectID, testCase, documentIDs)
		response.Cases = append(response.Cases, result)
		if result.Passed {
			response.Passed++
		}
		if result.Metrics.RouteCorrect {
			routeHits++
		}
		if result.Metrics.ExpectedCount > 0 {
			recallSum += result.Metrics.RecallAtK
			recallCases++
		}
		latencySum += result.Metrics.LatencyMS
	}
	if response.Total > 0 {
		response.PassRate = float64(response.Passed) / float64(response.Total)
		response.RouteAccuracy = float64(routeHits) / float64(response.Total)
		response.AverageLatencyMS = latencySum / int64(response.Total)
	}
	if recallCases > 0 {
		response.AverageRecallAtK = recallSum / float64(recallCases)
	}
	return response, nil
}

func (e *Evaluator) evaluateCase(
	ctx context.Context,
	userID, subjectID string,
	testCase config.EvaluationCase,
	documentIDs map[string]string,
) types.RetrievalEvaluationCaseResult {
	result := types.RetrievalEvaluationCaseResult{
		Name: testCase.Name, Query: testCase.Query, ExpectedRoute: testCase.ExpectedRoute,
		MissingDocuments: make([]string, 0),
	}
	expectedDocIDs := make([]string, 0, len(testCase.ExpectedDocuments))
	for _, filename := range testCase.ExpectedDocuments {
		if id := documentIDs[filename]; id != "" {
			expectedDocIDs = append(expectedDocIDs, id)
		} else {
			result.MissingDocuments = append(result.MissingDocuments, filename)
		}
	}

	startedAt := time.Now()
	analysis := chatflow.AnalyzeQuery(ctx, e.svcCtx, testCase.Query, e.svcCtx.Config.LLM.Provider, e.svcCtx.Config.LLM.Model)
	route := analysis.Route
	if route != chatflow.QueryRouteRAG {
		result.Metrics = chatflow.RouteMetrics(route, testCase.ExpectedRoute, startedAt, e.svcCtx.Config.Evaluation)
		result.Passed = result.Metrics.EvaluationPassed && len(result.MissingDocuments) == 0
		return result
	}

	chunks, metrics, err := NewService(e.svcCtx).SearchWithOptions(ctx, userID, subjectID, testCase.Query, SearchOptions{
		ExpectedDocIDs: expectedDocIDs,
		ExpectedRoute:  testCase.ExpectedRoute,
		SearchQuery:    analysis.SearchQuery,
	})
	_ = chunks
	result.Metrics = metrics
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	result.Passed = metrics.EvaluationPassed && len(result.MissingDocuments) == 0
	if strings.TrimSpace(testCase.ExpectedRoute) != "" && !strings.EqualFold(testCase.ExpectedRoute, string(route)) {
		result.Passed = false
	}
	return result
}
