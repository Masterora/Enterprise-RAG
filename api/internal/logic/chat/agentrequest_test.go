package chat

import (
	"testing"

	"enterprise-rag/api/internal/types"

	"github.com/google/uuid"
)

func TestAgentRequestUsesConfiguredTopKAndMapsFields(t *testing.T) {
	req := &types.ChatAskReq{
		SessionID: "session", SubjectID: "subject", Query: "question",
		LlmProvider: "openrouter", LlmModel: "model", WebSearch: true,
	}
	runID := uuid.NewString()
	result := agentRequest(runID, "tenant", "user", req, 8)
	if _, err := uuid.Parse(result.RunID); err != nil {
		t.Fatalf("run id is not a UUID: %q", result.RunID)
	}
	if result.RunID != runID {
		t.Fatalf("run id = %q, want %q", result.RunID, runID)
	}
	if result.UserID != "user" || result.SubjectID != "subject" || result.Question != "question" {
		t.Fatalf("unexpected request mapping: %+v", result)
	}
	if result.TopK != 8 || result.LLMModel != "model" || !result.WebSearch {
		t.Fatalf("unexpected request options: %+v", result)
	}
	if result.ExpectedDocIDs == nil || result.ExpectedChunkIDs == nil {
		t.Fatalf("expected id collections must be arrays: %+v", result)
	}
}

func TestAgentRequestKeepsExplicitTopK(t *testing.T) {
	result := agentRequest(uuid.NewString(), "tenant", "user", &types.ChatAskReq{TopK: 3}, 8)
	if result.TopK != 3 {
		t.Fatalf("top_k = %d, want 3", result.TopK)
	}
}
