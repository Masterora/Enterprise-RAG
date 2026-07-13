package retrieval

import (
	"context"
	"strings"
	"time"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/infrastructure/llm"
)

func (s *Service) rewriteQuery(ctx context.Context, query, provider, model string) (string, error) {
	timeout := s.svcCtx.Config.Retrieval.RewriteTimeoutSeconds
	if timeout <= 0 {
		timeout = 6
	}
	rewriteCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	prompt := strings.TrimSpace(s.svcCtx.Config.Prompt.QueryRewriteTemplate)
	if prompt == "" {
		prompt = `请把下面的问题改写成更适合知识库检索的查询句。
要求：
1. 保留原问题含义，不要回答问题。
2. 补充同义表达和关键实体，但不要编造新事实。
3. 只输出一行检索查询，不要解释，长度控制在 80 字以内。

原问题：{{question}}`
	}
	prompt = strings.ReplaceAll(prompt, "{{question}}", query)

	rewriteLLM := s.svcCtx.LLM
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if provider != "" || model != "" {
		override := config.ProviderConf{Provider: provider, Model: model, ApiKey: s.svcCtx.Config.LLM.ApiKey, BaseURL: s.svcCtx.Config.LLM.BaseURL}
		if override.Provider == "" {
			override.Provider = s.svcCtx.Config.LLM.Provider
		}
		if override.Model == "" {
			override.Model = s.svcCtx.Config.LLM.Model
		}
		var err error
		rewriteLLM, err = llm.NewClient(override)
		if err != nil {
			return "", err
		}
	}
	startedAt := time.Now()
	rewriteCtx = s.svcCtx.Metrics.ModelUsageContext(rewriteCtx, "llm", "query_rewrite",
		resolvedProvider(provider, s.svcCtx.Config.LLM.Provider))
	rewritten, err := rewriteLLM.Generate(rewriteCtx, prompt, false)
	s.svcCtx.Metrics.ObserveModel("llm", "query_rewrite", resolvedProvider(provider, s.svcCtx.Config.LLM.Provider), metricOutcome(err), time.Since(startedAt))
	if err != nil {
		return "", err
	}
	return sanitizeRewrittenQuery(query, rewritten), nil
}

func resolvedProvider(requested, fallback string) string {
	if requested = strings.TrimSpace(requested); requested != "" {
		return requested
	}
	return fallback
}

func sanitizeRewrittenQuery(original, rewritten string) string {
	rewritten = strings.Trim(strings.TrimSpace(rewritten), "`\"'“”‘’")
	rewritten = strings.Join(strings.Fields(strings.ReplaceAll(rewritten, "\n", " ")), " ")
	if rewritten == "" || strings.EqualFold(rewritten, "无法确定") {
		return original
	}
	if runes := []rune(rewritten); len(runes) > 120 {
		return string(runes[:120])
	}
	return rewritten
}
