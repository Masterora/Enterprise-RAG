package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"enterprise-rag/backend/internal/service/chatflow"
	retrievalsvc "enterprise-rag/backend/internal/service/retrieval"
	"enterprise-rag/backend/internal/svc"
)

const (
	ToolKnowledgeSearch    = "knowledge_search"
	ToolKnowledgeOverview  = "knowledge_overview"
	ToolDocumentNavigation = "document_navigation"
	ToolWebSearch          = "web_search"
)

func NewDefaultRegistry(svcCtx *svc.ServiceContext) (*Registry, error) {
	registry := NewRegistry()
	for _, tool := range []Tool{
		&knowledgeSearchTool{svcCtx: svcCtx},
		&knowledgeOverviewTool{svcCtx: svcCtx},
		&documentNavigationTool{svcCtx: svcCtx},
		&webSearchTool{},
	} {
		if err := registry.Register(tool); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

type knowledgeSearchTool struct {
	svcCtx *svc.ServiceContext
}

func (t *knowledgeSearchTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolKnowledgeSearch,
		Description: "在当前知识库中检索与问题相关的原文片段，适用于事实、解释、比较、流程和复合问题。",
		Parameters: objectSchema(map[string]any{
			"query": map[string]any{"type": "string", "description": "面向检索的完整查询"},
			"top_k": map[string]any{"type": "integer", "minimum": 1, "maximum": 20},
		}, "query"),
	}
}

func (t *knowledgeSearchTool) Execute(ctx context.Context, toolContext ToolContext, arguments json.RawMessage) (ToolResult, error) {
	var args struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := decodeArguments(arguments, &args); err != nil {
		return ToolResult{}, err
	}
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return ToolResult{}, errors.New("knowledge_search requires query")
	}
	if args.TopK <= 0 {
		args.TopK = toolContext.TopK
	} else if args.TopK > 20 {
		return ToolResult{}, errors.New("knowledge_search top_k must not exceed 20")
	}
	chunks, metrics, err := retrievalsvc.NewService(t.svcCtx).SearchWithOptions(ctx, toolContext.UserID, toolContext.SubjectID, toolContext.Question, retrievalsvc.SearchOptions{
		TopK:             args.TopK,
		ExpectedDocIDs:   toolContext.ExpectedDocs,
		ExpectedChunkIDs: toolContext.ExpectedIDs,
		LLMProvider:      toolContext.LLMProvider,
		LLMModel:         toolContext.LLMModel,
		SearchQuery:      args.Query,
	})
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Content: fmt.Sprintf("知识检索完成，找到 %d 个可用片段。", len(chunks)),
		Summary: fmt.Sprintf("找到 %d 个相关片段", len(chunks)),
		Chunks:  chunks,
		Metrics: metrics,
	}, nil
}

type knowledgeOverviewTool struct {
	svcCtx *svc.ServiceContext
}

func (t *knowledgeOverviewTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolKnowledgeOverview,
		Description: "汇总当前知识库的文档主题、覆盖范围和可支持的问题类型。",
		Parameters:  objectSchema(nil),
	}
}

func (t *knowledgeOverviewTool) Execute(ctx context.Context, toolContext ToolContext, arguments json.RawMessage) (ToolResult, error) {
	if err := decodeArguments(arguments, &struct{}{}); err != nil {
		return ToolResult{}, err
	}
	content, chunks, err := chatflow.BuildKnowledgeOverviewTool(ctx, t.svcCtx, toolContext.UserID, toolContext.SubjectID)
	return ToolResult{Content: content, Summary: fmt.Sprintf("已整理 %d 篇代表文档", len(chunks)), Chunks: chunks}, err
}

type documentNavigationTool struct {
	svcCtx *svc.ServiceContext
}

func (t *documentNavigationTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolDocumentNavigation,
		Description: "按主题定位当前知识库中的相关文档和章节，不负责回答文档中的具体事实。",
		Parameters: objectSchema(map[string]any{
			"topic": map[string]any{"type": "string", "description": "需要定位的主题或实体"},
		}, "topic"),
	}
}

func (t *documentNavigationTool) Execute(ctx context.Context, toolContext ToolContext, arguments json.RawMessage) (ToolResult, error) {
	var args struct {
		Topic string `json:"topic"`
	}
	if err := decodeArguments(arguments, &args); err != nil {
		return ToolResult{}, err
	}
	args.Topic = strings.TrimSpace(args.Topic)
	if args.Topic == "" {
		return ToolResult{}, errors.New("document_navigation requires topic")
	}
	content, chunks, err := chatflow.BuildDocumentNavigationTool(ctx, t.svcCtx, toolContext.UserID, toolContext.SubjectID, args.Topic)
	return ToolResult{Content: content, Summary: fmt.Sprintf("已定位 %d 篇相关文档", len(chunks)), Chunks: chunks}, err
}

type webSearchTool struct{}

func (t *webSearchTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolWebSearch,
		Description: "搜索公开网页，为当前问题补充知识库之外的最新信息和外部来源。",
		Parameters: objectSchema(map[string]any{
			"query": map[string]any{"type": "string", "description": "外部网页搜索查询"},
		}, "query"),
	}
}

func (t *webSearchTool) Execute(ctx context.Context, toolContext ToolContext, arguments json.RawMessage) (ToolResult, error) {
	if !toolContext.WebSearch {
		return ToolResult{}, errors.New("web_search is not enabled for this request")
	}
	var args struct {
		Query string `json:"query"`
	}
	if err := decodeArguments(arguments, &args); err != nil {
		return ToolResult{}, err
	}
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return ToolResult{}, errors.New("web_search requires query")
	}
	links, err := toolContext.LLM.SearchWeb(ctx, args.Query)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Content:       fmt.Sprintf("联网搜索完成，找到 %d 个可用网页来源。", len(links)),
		Summary:       fmt.Sprintf("找到 %d 个外部网页来源", len(links)),
		ExternalLinks: links,
	}, nil
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func decodeArguments(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid tool arguments: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("invalid tool arguments: multiple JSON values")
	}
	return nil
}
