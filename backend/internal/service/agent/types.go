package agent

import (
	"context"
	"encoding/json"

	"enterprise-rag/backend/internal/infrastructure/llm"
	"enterprise-rag/backend/internal/types"
)

type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type Plan struct {
	Goal   string     `json:"goal"`
	Done   bool       `json:"done"`
	Reason string     `json:"reason,omitempty"`
	Tools  []ToolCall `json:"tools"`
}

type ToolContext struct {
	UserID       string
	SubjectID    string
	Question     string
	TopK         int
	WebSearch    bool
	LLMProvider  string
	LLMModel     string
	LLM          llm.Client
	ExpectedDocs []string
	ExpectedIDs  []string
}

type ToolResult struct {
	Content       string
	Summary       string
	Chunks        []types.RetrievalChunk
	ExternalLinks []types.ExternalLink
	Metrics       types.RetrievalMetrics
}

type Tool interface {
	Spec() ToolSpec
	Execute(ctx context.Context, toolContext ToolContext, arguments json.RawMessage) (ToolResult, error)
}

type Input struct {
	SessionID        string
	MessageID        string
	UserID           string
	SubjectID        string
	Question         string
	TopK             int
	LLMProvider      string
	LLMModel         string
	WebSearch        bool
	ExpectedDocIDs   []string
	ExpectedChunkIDs []string
	ExpectedRoute    string
	ExpectedOutcome  string
}

type Callbacks struct {
	OnStatus     func(string) error
	OnStep       func(types.AgentStep) error
	OnSources    func([]types.RetrievalChunk) error
	OnWebSources func([]types.ExternalLink) error
	OnMetrics    func(types.RetrievalMetrics) error
	OnDelta      func(string) error
}

type Output struct {
	Answer        string
	Chunks        []types.RetrievalChunk
	ExternalLinks []types.ExternalLink
	Metrics       types.RetrievalMetrics
	Steps         []types.AgentStep
}
