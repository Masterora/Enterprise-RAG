package knowledge

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"
)

type overviewDocSummary struct {
	document   model.Document
	sections   []string
	chunk      *types.RetrievalChunk
	chunkScore float64
	count      int
}

const (
	overviewCatalogCharacterBudget = 11000
	overviewEvidenceLimit          = 5
)

type OverviewResult struct {
	Content          string
	Chunks           []types.RetrievalChunk
	TotalDocuments   int
	CoveredDocuments int
}

func BuildKnowledgeOverviewTool(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	tenantID, userID, subjectID string,
) (OverviewResult, error) {
	subject, err := svcCtx.SubjectRepo.GetAccessibleByID(ctx, subjectID, userID, tenantID)
	if err != nil {
		return OverviewResult{}, err
	}

	documents, _, err := svcCtx.DocumentRepo.ListByUser(ctx, model.DocumentListFilter{
		UserID:         userID,
		TenantID:       tenantID,
		AllTenantUsers: true,
		SubjectID:      subjectID,
		Status:         model.DocumentStatusIndexed,
		PageSize:       200,
	})
	if err != nil {
		return OverviewResult{}, err
	}
	if len(documents) == 0 {
		return OverviewResult{
			Content: "知识库“" + subject.Name + "”当前暂无已索引文档。",
		}, nil
	}

	chunks, err := svcCtx.ChunkRepo.ListBySubject(ctx, tenantID, subjectID)
	if err != nil {
		return OverviewResult{}, err
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
		if section != "" && len(summary.sections) < 8 && !containsString(summary.sections, section) {
			summary.sections = append(summary.sections, section)
		}
		score := overviewChunkScore(chunk.Section, chunk.Content, chunk.ChunkIndex)
		if strings.TrimSpace(chunk.Content) != "" && (summary.chunk == nil || score > summary.chunkScore) {
			summary.chunk = &types.RetrievalChunk{
				ID:         chunk.ID,
				TenantID:   chunk.TenantID,
				DocID:      chunk.DocID,
				DocName:    summary.document.Filename,
				SubjectID:  chunk.SubjectID,
				UserID:     chunk.UserID,
				ChunkIndex: int64(chunk.ChunkIndex),
				Page:       int64(chunk.Page),
				Section:    chunk.Section,
				Content:    chunk.Content,
				Source:     "overview",
			}
			summary.chunkScore = score
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
		return OverviewResult{
			Content:        fmt.Sprintf("知识库“%s”当前暂无可用于回答的已解析内容。", subject.Name),
			TotalDocuments: len(documents),
		}, nil
	}

	limit := minInt(len(ordered), overviewEvidenceLimit)
	citations := make([]types.RetrievalChunk, 0, limit)
	for index := 0; index < limit; index++ {
		citations = append(citations, *ordered[index].chunk)
	}

	draft := buildStructuredKnowledgeOverview(subject.Name, len(documents), ordered)
	return OverviewResult{
		Content:          formatOverviewAnswer(draft),
		Chunks:           citations,
		TotalDocuments:   len(documents),
		CoveredDocuments: len(ordered),
	}, nil
}

func buildStructuredKnowledgeOverview(subjectName string, documentCount int, summaries []*overviewDocSummary) string {
	coveredCount := len(summaries)
	coverage := fmt.Sprintf("本次概览已覆盖其中 %d 篇。", coveredCount)
	if coveredCount == documentCount {
		coverage = fmt.Sprintf("本次概览已覆盖全部 %d 篇。", documentCount)
	}
	lines := make([]string, 0, len(summaries)+2)
	lines = append(lines, fmt.Sprintf(
		"知识库“%s”当前共包含 %d 篇已索引文档，%s以下内容按全部可用文档整理：",
		subjectName,
		documentCount,
		coverage,
	))
	perDocumentBudget := overviewCatalogCharacterBudget / maxInt(coveredCount, 1)
	titleLimit := minInt(48, maxInt(16, perDocumentBudget/3))
	descriptionLimit := minInt(220, maxInt(24, perDocumentBudget-titleLimit-16))
	for index, summary := range summaries {
		title := shortenOverviewTitle(buildOverviewTitle(summary), titleLimit)
		description := compactRunes(buildOverviewDescription(summary), descriptionLimit)
		lines = append(lines, fmt.Sprintf("%d. %s：%s", index+1, title, description))
	}
	return normalizeAnswerText(strings.Join(lines, "\n"))
}

func formatOverviewAnswer(answer string) string {
	answer = normalizeAnswerText(answer)
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

func buildOverviewTitle(summary *overviewDocSummary) string {
	title := strings.TrimSuffix(summary.document.Filename, filepath.Ext(summary.document.Filename))
	return shortenOverviewTitle(title, 60)
}

func buildOverviewDescription(summary *overviewDocSummary) string {
	if summary.chunk == nil {
		return "可进一步查看对应文档内容。"
	}
	content := compactRunes(strings.TrimSpace(summary.chunk.Content), 220)
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, buildOverviewTitle(summary))
	content = strings.TrimLeft(content, "：:，,。;；、 ")
	content = trimRepeatedLead(content)
	sections := cleanOverviewSections(summary.sections)
	if sections != "" {
		content = fmt.Sprintf("代表章节：%s。代表内容：%s", compactRunes(sections, 100), content)
	}
	if strings.TrimSpace(content) == "" {
		return "可进一步查看对应文档内容。"
	}
	return content
}

func overviewChunkScore(section, content string, chunkIndex int) float64 {
	contentLength := len([]rune(strings.TrimSpace(content)))
	if contentLength == 0 {
		return -1
	}
	score := float64(minInt(contentLength, 1000)) / 100
	if !isWeakSection(section) && !isMeaninglessTheme(trimSectionPrefix(section)) {
		score += 12
	}
	if chunkIndex < 5 {
		score += float64(5 - chunkIndex)
	}
	return score
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
