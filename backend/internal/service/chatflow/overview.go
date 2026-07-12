package chatflow

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/infrastructure/llm"
	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type overviewDocSummary struct {
	document model.Document
	sections []string
	chunk    *types.RetrievalChunk
	count    int
}

func buildKnowledgeOverview(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, subjectID, query, llmProvider, llmModel string,
) (string, []types.RetrievalChunk, error) {
	subject, err := svcCtx.SubjectRepo.GetAccessibleByID(ctx, subjectID, userID)
	if err != nil {
		return "", nil, err
	}

	documents, _, err := svcCtx.DocumentRepo.ListByUser(ctx, model.DocumentListFilter{
		UserID:    userID,
		SubjectID: subjectID,
		Status:    model.DocumentStatusIndexed,
		PageSize:  200,
	})
	if err != nil {
		return "", nil, err
	}
	if len(documents) == 0 {
		return fmt.Sprintf("知识库“%s”当前暂无已索引文档。", subject.Name), nil, nil
	}

	chunks, err := svcCtx.ChunkRepo.ListBySubject(ctx, subjectID)
	if err != nil {
		return "", nil, err
	}

	summaries := make(map[string]*overviewDocSummary, len(documents))
	for _, document := range documents {
		doc := document
		summaries[document.ID] = &overviewDocSummary{document: doc}
	}

	for _, chunk := range chunks {
		summary, ok := summaries[chunk.DocID]
		if !ok {
			continue
		}
		summary.count++
		section := strings.TrimSpace(chunk.Section)
		if isWeakSection(section) {
			section = ""
		}
		if section != "" && len(summary.sections) < 3 && !containsString(summary.sections, section) {
			summary.sections = append(summary.sections, section)
		}
		if summary.chunk == nil && strings.TrimSpace(chunk.Content) != "" {
			summary.chunk = &types.RetrievalChunk{
				ID:         chunk.ID,
				DocID:      chunk.DocID,
				DocName:    summary.document.Filename,
				SubjectID:  chunk.SubjectID,
				UserID:     chunk.UserID,
				ChunkIndex: int64(chunk.ChunkIndex),
				Page:       int64(chunk.Page),
				Section:    chunk.Section,
				Content:    chunk.Content,
				Score:      1,
				RawScore:   1,
				Source:     "router",
			}
		}
	}

	ordered := make([]*overviewDocSummary, 0, len(summaries))
	for _, summary := range summaries {
		if summary.chunk == nil {
			continue
		}
		ordered = append(ordered, summary)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].count == ordered[j].count {
			return ordered[i].document.CreatedAt.After(ordered[j].document.CreatedAt)
		}
		return ordered[i].count > ordered[j].count
	})

	if len(ordered) == 0 {
		return fmt.Sprintf("知识库“%s”当前暂无可用于回答的已解析内容。", subject.Name), nil, nil
	}

	unique := dedupeOverviewSummaries(ordered)
	themes := collectOverviewThemes(unique, 4)
	limit := minInt(len(unique), 4)
	citations := make([]types.RetrievalChunk, 0, limit)
	for index := 0; index < limit; index++ {
		citations = append(citations, *unique[index].chunk)
	}

	draft := buildStructuredKnowledgeOverview(subject.Name, len(documents), themes, unique[:limit], citations)
	if overview, err := polishKnowledgeOverview(ctx, svcCtx, query, draft, unique[:limit], llmProvider, llmModel); err == nil && strings.TrimSpace(overview) != "" {
		return formatOverviewAnswer(strings.TrimSpace(overview)), citations, nil
	} else if err != nil {
		logx.WithContext(ctx).Errorf("knowledge overview llm failed: subject_id=%s err=%v", subjectID, err)
	}
	return formatOverviewAnswer(draft), citations, nil
}

func dedupeOverviewSummaries(items []*overviewDocSummary) []*overviewDocSummary {
	seen := make(map[string]struct{}, len(items))
	result := make([]*overviewDocSummary, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(strings.ToLower(item.document.Filename))
		if key == "" {
			key = item.document.ID
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func collectOverviewThemes(items []*overviewDocSummary, limit int) []string {
	if limit <= 0 {
		limit = 4
	}
	counts := make(map[string]int)
	ordered := make([]string, 0)
	for _, item := range items {
		for _, section := range item.sections {
			section = strings.TrimSpace(section)
			if isWeakSection(section) {
				continue
			}
			if _, ok := counts[section]; !ok {
				ordered = append(ordered, section)
			}
			counts[section]++
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if counts[ordered[i]] == counts[ordered[j]] {
			return len([]rune(ordered[i])) < len([]rune(ordered[j]))
		}
		return counts[ordered[i]] > counts[ordered[j]]
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	return ordered
}

func polishKnowledgeOverview(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	query string,
	draft string,
	summaries []*overviewDocSummary,
	llmProvider, llmModel string,
) (string, error) {
	client, err := resolveRouteLLM(svcCtx, llmProvider, llmModel)
	if err != nil {
		return "", err
	}

	formattedSummaries := make([]overviewDocSummary, 0, len(summaries))
	for _, summary := range summaries {
		formattedSummaries = append(formattedSummaries, *summary)
	}
	answer, err := GenerateAnswer(ctx, client, svcCtx.Config.Reliability,
		BuildOverviewPolishPrompt(svcCtx.Config.Prompt, query, draft, formattedSummaries))
	if err != nil {
		return "", err
	}
	return formatOverviewAnswer(answer), nil
}

func resolveRouteLLM(svcCtx *svc.ServiceContext, provider, model string) (llm.Client, error) {
	override := config.ProviderConf{
		Provider: strings.TrimSpace(provider),
		Model:    strings.TrimSpace(model),
		ApiKey:   svcCtx.Config.LLM.ApiKey,
		BaseURL:  svcCtx.Config.LLM.BaseURL,
	}
	if override.Provider == "" {
		override.Provider = svcCtx.Config.LLM.Provider
	}
	if override.Model == "" {
		override.Model = svcCtx.Config.LLM.Model
	}
	if strings.EqualFold(override.Provider, strings.TrimSpace(svcCtx.Config.LLM.Provider)) &&
		strings.TrimSpace(override.Model) == strings.TrimSpace(svcCtx.Config.LLM.Model) {
		return svcCtx.LLM, nil
	}
	return llm.NewClient(override)
}

func buildStructuredKnowledgeOverview(subjectName string, documentCount int, themes []string, summaries []*overviewDocSummary, citations []types.RetrievalChunk) string {
	lines := make([]string, 0, len(summaries)+2)
	lines = append(lines, buildOverviewLead(subjectName, documentCount, themes, citations))
	for index, summary := range summaries {
		title := buildOverviewTitle(summary)
		description := buildOverviewDescription(summary)
		lines = append(lines, fmt.Sprintf("%d. %s：%s [引用%d]", index+1, title, description, index+1))
	}
	return NormalizeAnswerText(strings.Join(lines, "\n"))
}

func formatOverviewAnswer(answer string) string {
	answer = NormalizeAnswerText(answer)
	lines := strings.Split(answer, "\n")
	for index, line := range lines {
		matches := overviewListLinePattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) != 5 {
			continue
		}
		title := strings.TrimSpace(matches[2])
		description := strings.TrimSpace(matches[4])
		if title == "" || description == "" || strings.Contains(title, "**") {
			continue
		}
		lines[index] = fmt.Sprintf("%s **%s**：%s", matches[1], shortenOverviewTitle(title, 32), description)
	}
	return strings.Join(lines, "\n")
}

func buildOverviewLead(subjectName string, documentCount int, themes []string, citations []types.RetrievalChunk) string {
	lead := fmt.Sprintf("知识库“%s”当前共包含 %d 篇已索引文档。", subjectName, documentCount)
	if len(themes) > 0 {
		lead = fmt.Sprintf("知识库“%s”当前共包含 %d 篇已索引文档，主要覆盖 %s 等方向。", subjectName, documentCount, strings.Join(themes, "、"))
	}
	if refs := buildOverviewReferenceSuffix(citations, minInt(len(citations), 3)); refs != "" {
		lead = lead + " " + refs
	}
	return lead
}

func buildOverviewReferenceSuffix(citations []types.RetrievalChunk, limit int) string {
	if limit <= 0 {
		return ""
	}
	refs := make([]string, 0, limit)
	for index := 0; index < limit && index < len(citations); index++ {
		refs = append(refs, fmt.Sprintf("[引用%d]", index+1))
	}
	return strings.Join(refs, " ")
}

func buildOverviewTitle(summary *overviewDocSummary) string {
	title := cleanOverviewSections(summary.sections)
	if title == "" {
		title = strings.TrimSuffix(summary.document.Filename, filepath.Ext(summary.document.Filename))
	}
	return shortenOverviewTitle(title, 22)
}

func buildOverviewDescription(summary *overviewDocSummary) string {
	if summary.chunk == nil {
		return "可进一步查看对应文档内容。"
	}
	content := compactRunes(strings.TrimSpace(summary.chunk.Content), 150)
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, buildOverviewTitle(summary))
	content = strings.TrimLeft(content, "：:，,。;；、 ")
	content = trimRepeatedLead(content)
	if content == "" {
		return "可进一步查看对应文档内容。"
	}
	return content
}

func shortenOverviewTitle(title string, limit int) string {
	title = strings.TrimSpace(title)
	for _, sep := range []string{"：", ":", "，", ",", "、", "（", "("} {
		if head, _, ok := strings.Cut(title, sep); ok && strings.TrimSpace(head) != "" {
			title = strings.TrimSpace(head)
			break
		}
	}
	runes := []rune(title)
	if len(runes) <= limit {
		return title
	}
	return string(runes[:limit]) + "..."
}

func trimRepeatedLead(text string) string {
	for _, prefix := range []string{
		"说明", "主要说明", "详细定义", "重点说明", "给出", "规定", "用于", "描述", "介绍",
	} {
		if strings.HasPrefix(text, prefix) && len([]rune(text)) > len([]rune(prefix))+2 {
			return strings.TrimLeft(strings.TrimPrefix(text, prefix), "：:，,。;；、 ")
		}
	}
	return text
}

var overviewListLinePattern = regexp.MustCompile(`^(\d+[.．、\)）])\s*(.+?)([：:])\s*(.+)$`)

func cleanOverviewSections(sections []string) string {
	cleaned := make([]string, 0, len(sections))
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" || isWeakSection(section) {
			continue
		}
		section = trimSectionPrefix(section)
		if section == "" || isMeaninglessTheme(section) {
			continue
		}
		if !containsString(cleaned, section) {
			cleaned = append(cleaned, section)
		}
	}
	return strings.Join(cleaned, "、")
}

func trimSectionPrefix(section string) string {
	section = strings.TrimSpace(section)
	prefixes := []string{"第", "一、", "二、", "三、", "四、", "五、", "六、", "七、", "八、", "九、", "十、"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(section, prefix) && len([]rune(section)) > len([]rune(prefix))+1 {
			section = strings.TrimSpace(strings.TrimPrefix(section, prefix))
			break
		}
	}
	for len(section) > 0 {
		r := []rune(section)[0]
		if unicode.IsDigit(r) || r == '.' || r == '．' || r == '、' || r == ' ' || r == '\t' {
			section = strings.TrimSpace(string([]rune(section)[1:]))
			continue
		}
		break
	}
	return section
}

func isMeaninglessTheme(section string) bool {
	if section == "" {
		return true
	}
	digitCount := 0
	for _, r := range section {
		if unicode.IsDigit(r) || r == '.' || r == '．' {
			digitCount++
		}
	}
	return digitCount >= len([]rune(section))/2
}

func compactRunes(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "..."
}
