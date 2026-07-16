package knowledge

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"
)

func buildDocumentNavigation(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	tenantID, userID, subjectID, query string,
) (string, []types.RetrievalChunk, error) {
	documents, _, err := svcCtx.DocumentRepo.ListByUser(ctx, model.DocumentListFilter{
		UserID:         userID,
		TenantID:       tenantID,
		AllTenantUsers: true,
		SubjectID:      subjectID,
		Status:         model.DocumentStatusIndexed,
		PageSize:       200,
	})
	if err != nil {
		return "", nil, err
	}
	if len(documents) == 0 {
		return "当前知识库暂无已索引文档。", nil, nil
	}

	chunks, err := svcCtx.ChunkRepo.ListBySubject(ctx, tenantID, subjectID)
	if err != nil {
		return "", nil, err
	}

	documentsByID := make(map[string]model.Document, len(documents))
	for _, document := range documents {
		documentsByID[document.ID] = document
	}

	topic := extractNavigationTopic(query)
	type matchSummary struct {
		document model.Document
		score    float64
		section  string
		chunk    *types.RetrievalChunk
	}
	matches := make(map[string]*matchSummary)

	for _, chunk := range chunks {
		document, ok := documentsByID[chunk.DocID]
		if !ok {
			continue
		}
		score := routedChunkScore(topic, document.Filename, chunk.Section, chunk.Content)
		if score <= 0 {
			continue
		}
		current := matches[chunk.DocID]
		if current == nil || score > current.score {
			matches[chunk.DocID] = &matchSummary{
				document: document,
				score:    score,
				section:  strings.TrimSpace(chunk.Section),
				chunk: &types.RetrievalChunk{
					ID:         chunk.ID,
					TenantID:   chunk.TenantID,
					DocID:      chunk.DocID,
					DocName:    document.Filename,
					SubjectID:  chunk.SubjectID,
					UserID:     chunk.UserID,
					ChunkIndex: int64(chunk.ChunkIndex),
					Page:       int64(chunk.Page),
					Section:    chunk.Section,
					Content:    chunk.Content,
					Source:     "navigation",
				},
			}
		}
	}

	ordered := make([]*matchSummary, 0, len(matches))
	for _, match := range matches {
		ordered = append(ordered, match)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].score == ordered[j].score {
			return ordered[i].document.CreatedAt.After(ordered[j].document.CreatedAt)
		}
		return ordered[i].score > ordered[j].score
	})

	if len(ordered) == 0 {
		return fmt.Sprintf("未找到与“%s”直接相关的已索引文档。", topic), nil, nil
	}

	limit := minInt(len(ordered), 6)
	citations := make([]types.RetrievalChunk, 0, limit)
	lines := make([]string, 0, limit+1)
	lines = append(lines, fmt.Sprintf("与“%s”直接相关的已索引文档有 %d 篇：", topic, len(ordered)))
	for index := 0; index < limit; index++ {
		match := ordered[index]
		citations = append(citations, *match.chunk)
		sectionText := "相关内容"
		if !isWeakSection(match.section) && match.section != "" {
			sectionText = match.section
		}
		lines = append(lines, fmt.Sprintf("%d. %s：可优先查看 %s。[引用%d]", index+1, match.document.Filename, sectionText, index+1))
	}
	return strings.Join(lines, "\n"), citations, nil
}

func BuildDocumentNavigationTool(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	tenantID, userID, subjectID, topic string,
) (string, []types.RetrievalChunk, error) {
	return buildDocumentNavigation(ctx, svcCtx, tenantID, userID, subjectID, topic)
}

func extractNavigationTopic(query string) string {
	topic := strings.Join(strings.Fields(query), " ")
	if topic == "" {
		return "当前问题"
	}
	return topic
}

func routedChunkScore(topic, filename, section, content string) float64 {
	topic = routeNormalizeText(topic)
	filename = routeNormalizeText(filename)
	section = routeNormalizeText(section)
	content = routeNormalizeText(content)
	if topic == "" {
		return 0
	}

	var score float64
	if strings.Contains(filename, topic) {
		score += 6
	}
	if strings.Contains(section, topic) {
		score += 5
	}
	if strings.Contains(content, topic) {
		score += 2
	}
	for _, token := range routeKeywordTokens(topic) {
		if strings.Contains(filename, token) {
			score += 3
		}
		if strings.Contains(section, token) {
			score += 2
		}
		if strings.Contains(content, token) {
			score += 1
		}
	}
	return score
}

func routeKeywordTokens(text string) []string {
	text = routeNormalizeText(text)
	tokens := make([]string, 0)
	var builder strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			continue
		}
		if builder.Len() > 0 {
			tokens = appendRouteToken(tokens, builder.String())
			builder.Reset()
		}
	}
	if builder.Len() > 0 {
		tokens = appendRouteToken(tokens, builder.String())
	}
	return tokens
}

func appendRouteToken(tokens []string, token string) []string {
	runes := []rune(token)
	if len(runes) < 2 {
		return tokens
	}
	if routeContainsHan(runes) {
		for i := 0; i+1 < len(runes); i++ {
			tokens = append(tokens, string(runes[i:i+2]))
		}
		tokens = append(tokens, token)
		return tokens
	}
	return append(tokens, token)
}

func routeContainsHan(runes []rune) bool {
	for _, r := range runes {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func routeNormalizeText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer("？", "", "?", "", "。", "", ".", "", "，", "", ",", "", "：", "", ":", "", "（", "", "）", "", "(", "", ")", "")
	return replacer.Replace(text)
}
