package knowledge

import (
	"fmt"
	"strings"
	"testing"

	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/types"
)

func TestBuildStructuredKnowledgeOverviewCoversEveryDocument(t *testing.T) {
	summaries := make([]*overviewDocSummary, 0, 11)
	for index := 1; index <= 11; index++ {
		name := fmt.Sprintf("制度-%02d.md", index)
		summaries = append(summaries, &overviewDocSummary{
			document: model.Document{Filename: name},
			sections: []string{"适用范围", "执行要求"},
			chunk: &types.RetrievalChunk{
				DocName: name,
				Content: fmt.Sprintf("第%d篇文档说明对应制度的适用范围、执行要求和责任边界。", index),
			},
		})
	}

	result := buildStructuredKnowledgeOverview("测试知识库", 11, summaries)

	if !strings.Contains(result, "已覆盖全部 11 篇") {
		t.Fatalf("overview does not report complete coverage: %s", result)
	}
	if !strings.Contains(result, "制度-01") || !strings.Contains(result, "制度-11") {
		t.Fatalf("overview does not include every document: %s", result)
	}
}

func TestBuildStructuredKnowledgeOverviewStaysWithinCatalogBudget(t *testing.T) {
	summaries := make([]*overviewDocSummary, 0, 200)
	for index := 1; index <= 200; index++ {
		name := fmt.Sprintf("企业知识文档-%03d-较长文件名称.md", index)
		summaries = append(summaries, &overviewDocSummary{
			document: model.Document{Filename: name},
			chunk: &types.RetrievalChunk{
				DocName: name,
				Content: strings.Repeat("这是用于概览压缩测试的文档内容。", 30),
			},
		})
	}

	result := buildStructuredKnowledgeOverview("容量测试", 200, summaries)

	if length := len([]rune(result)); length > overviewCatalogCharacterBudget+1000 {
		t.Fatalf("overview length = %d, want <= %d", length, overviewCatalogCharacterBudget+1000)
	}
	if !strings.Contains(result, "企业知识文档-200") {
		t.Fatal("overview omitted the last document")
	}
}
