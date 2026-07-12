package chatflow

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/infrastructure/llm"
	"enterprise-rag/backend/internal/model"
	chatpresenter "enterprise-rag/backend/internal/presenter/chat"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

func NormalizeAnswerText(answer string) string {
	return strings.TrimSpace(normalizeLooseAnswer(answer))
}

func normalizeLooseAnswer(answer string) string {
	lines := strings.Split(strings.ReplaceAll(answer, "\r\n", "\n"), "\n")
	normalized := make([]string, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			if len(normalized) == 0 || normalized[len(normalized)-1] == "" {
				continue
			}
			normalized = append(normalized, "")
			continue
		}

		if len(normalized) > 0 {
			prev := normalized[len(normalized)-1]
			if prev != "" && numberedLineOnly(prev) {
				normalized[len(normalized)-1] = prev + " " + line
				continue
			}
		}

		normalized = append(normalized, line)
	}

	text := strings.Join(normalized, "\n")
	text = collapseBoilerplateHeadings(text)
	text = brokenNumberedLinePattern.ReplaceAllString(text, "$1$2 $3")
	text = strings.ReplaceAll(text, "：\n", "：")
	text = strings.ReplaceAll(text, ":\n", ":")
	text = mergeDetachedCitationLines(text)
	text = strings.ReplaceAll(text, "\n\n", "\n")
	return strings.TrimSpace(text)
}

func numberedLineOnly(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	runes := []rune(line)
	index := 0
	for index < len(runes) && unicode.IsDigit(runes[index]) {
		index++
	}
	if index == 0 || index >= len(runes) {
		return false
	}
	switch runes[index] {
	case '.', '．', '、', ')', '）':
		index++
	default:
		return false
	}
	for index < len(runes) {
		if !unicode.IsSpace(runes[index]) {
			return false
		}
		index++
	}
	return true
}

func mergeDetachedCitationLines(text string) string {
	return citationLinePattern.ReplaceAllString(text, " $1")
}

func collapseBoilerplateHeadings(text string) string {
	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if boilerplateHeadingPattern.MatchString(trimmed) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return strings.Join(filtered, "\n")
}

var citationLinePattern = regexp.MustCompile(`\n+[ \t]*(\[(?:引用|外链)?\d+\][^\n]*)`)
var boilerplateHeadingPattern = regexp.MustCompile(`^(核心结论|补充说明(?:如下)?|说明如下|总结|结论)[：:]?$`)
var brokenNumberedLinePattern = regexp.MustCompile(`(^|\n)(\d+[.．、\)）])\s*\n+([^\n]+)`)

func ResolveLLM(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ChatAskReq) (llm.Client, error) {
	override := config.ProviderConf{
		Provider: strings.TrimSpace(req.LlmProvider),
		Model:    strings.TrimSpace(req.LlmModel),
		ApiKey:   svcCtx.Config.LLM.ApiKey,
		BaseURL:  svcCtx.Config.LLM.BaseURL,
	}
	if override.Provider == "" {
		override.Provider = svcCtx.Config.LLM.Provider
	}
	if override.Model == "" {
		override.Model = svcCtx.Config.LLM.Model
	}
	if strings.EqualFold(strings.TrimSpace(override.Provider), strings.TrimSpace(svcCtx.Config.LLM.Provider)) &&
		strings.TrimSpace(override.Model) == strings.TrimSpace(svcCtx.Config.LLM.Model) {
		logx.WithContext(ctx).Infof("chat llm resolved: provider=%s model=%s reused_default=true", override.Provider, override.Model)
		return svcCtx.LLM, nil
	}
	logx.WithContext(ctx).Infof("chat llm resolved: provider=%s model=%s reused_default=false", override.Provider, override.Model)
	return llm.NewClient(override)
}

func PersistTurn(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.ChatAskReq,
	answer string,
	chunks []types.RetrievalChunk,
	externalLinks []types.ExternalLink,
	metrics types.RetrievalMetrics,
) error {
	sessionID := strings.TrimSpace(req.SessionID)
	messageID := strings.TrimSpace(req.MessageID)
	if sessionID == "" && messageID == "" {
		return nil
	}
	if _, err := uuid.Parse(sessionID); err != nil {
		return errors.New("invalid session id")
	}
	if _, err := uuid.Parse(messageID); err != nil {
		return errors.New("invalid message id")
	}

	citations, err := json.Marshal(chunks)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(chatpresenter.MessageMetadata{
		Metrics: metrics, ModelLabel: req.LlmModel, ModelID: req.LlmModel, WebSearch: req.WebSearch, ExternalLinks: externalLinks,
	})
	if err != nil {
		return err
	}

	now := time.Now()
	return svcCtx.ChatRepo.SaveTurn(ctx, &model.ChatSession{
		ID:          sessionID,
		UserID:      userID,
		SubjectID:   req.SubjectID,
		Title:       BuildSessionTitle(req.Query),
		LLMProvider: req.LlmProvider,
		LLMModel:    req.LlmModel,
	}, &model.ChatMessage{
		ID:        messageID,
		SessionID: sessionID,
		UserID:    userID,
		Question:  req.Query,
		Answer:    answer,
		Citations: citations,
		Metadata:  metadata,
		CreatedAt: now,
	})
}

func BuildSessionTitle(question string) string {
	runes := []rune(strings.TrimSpace(question))
	if len(runes) <= 18 {
		return string(runes)
	}
	return string(runes[:18]) + "..."
}
