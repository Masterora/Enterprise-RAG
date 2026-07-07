package chat

import (
	"fmt"
	"strings"

	"enterprise-rag/backend/internal/types"
)

func buildPrompt(query string, chunks []types.RetrievalChunk) string {
	var builder strings.Builder
	builder.WriteString("你是企业知识库问答助手。请严格基于给定资料回答问题。\n")
	builder.WriteString("回答规则：\n")
	builder.WriteString("1. 只能使用资料片段中的事实，不要编造。\n")
	builder.WriteString("2. 如果资料不足以回答，必须只回答：无法确定。\n")
	builder.WriteString("3. 答案应简洁、准确，必要时用中文分点说明。\n")
	builder.WriteString("4. 不要输出资料片段之外的来源。\n\n")
	builder.WriteString("问题：\n")
	builder.WriteString(query)
	builder.WriteString("\n\n资料片段：\n")

	for index, chunk := range chunks {
		builder.WriteString(fmt.Sprintf("[引用%d]\n文档：%s\n页码：%s\n章节：%s\n内容：%s\n\n",
			index+1,
			emptyAsUnknown(chunk.DocName),
			pageText(chunk.Page),
			emptyAsUnknown(chunk.Section),
			chunk.Content,
		))
	}

	builder.WriteString("请直接给出最终答案。")
	return builder.String()
}

func emptyAsUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未知"
	}
	return value
}

func pageText(page int64) string {
	if page <= 0 {
		return "无"
	}
	return fmt.Sprintf("%d", page)
}
