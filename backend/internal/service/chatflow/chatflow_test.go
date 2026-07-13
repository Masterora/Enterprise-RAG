package chatflow

import (
	"strings"
	"testing"

	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/types"
)

func TestIsNoAnswerSupportsInterfaceLanguages(t *testing.T) {
	for _, answer := range []string{"无法确定。", "Unable to determine.", "Cannot determine", "判断できません。"} {
		if !IsNoAnswer(answer) {
			t.Fatalf("expected no-answer response: %q", answer)
		}
	}
	if IsNoAnswer("资料显示该流程可执行。") {
		t.Fatal("valid answer was classified as no-answer")
	}
}

func TestExtractNavigationTopic(t *testing.T) {
	if got := extractNavigationTopic("向量数据库选型"); got != "向量数据库选型" {
		t.Fatalf("extractNavigationTopic() = %q, want %q", got, "向量数据库选型")
	}
}

func TestNormalizeAnswerTextMergesBrokenNumberedLines(t *testing.T) {
	input := "主要包含以下几个主题：\n\n1.\n\n基于 MQTT 的配电物联网通信协议与架构\n\n说明内容。 [引用1]"
	got := NormalizeAnswerText(input)
	want := "主要包含以下几个主题：1. 基于 MQTT 的配电物联网通信协议与架构\n说明内容。 [引用1]"
	if got != want {
		t.Fatalf("NormalizeAnswerText() = %q, want %q", got, want)
	}
}

func TestNormalizeAnswerTextDropsBoilerplateHeadingAndDetachedCitationLine(t *testing.T) {
	input := "核心结论：\nJaeger 是分布式追踪系统。\n[引用1]\n\n补充说明如下：\n可用于排查慢请求。\n[引用2]"
	got := NormalizeAnswerText(input)
	want := "Jaeger 是分布式追踪系统。 [引用1]\n可用于排查慢请求。 [引用2]"
	if got != want {
		t.Fatalf("NormalizeAnswerText() = %q, want %q", got, want)
	}
}

func TestNormalizeAnswerTextMergesMultipleStandaloneCitationLines(t *testing.T) {
	input := "该知识库围绕配电物联网协议展开。\n\n[引用1]\n\n[引用2]\n\n下一段说明。"
	got := NormalizeAnswerText(input)
	want := "该知识库围绕配电物联网协议展开。 [引用1] [引用2]\n下一段说明。"
	if got != want {
		t.Fatalf("NormalizeAnswerText() = %q, want %q", got, want)
	}
}

func TestFormatOverviewAnswerHighlightsListTitle(t *testing.T) {
	input := "该知识库主要覆盖协议与数据规范。\n1. MQTT 报文与 Topic 设计：详细定义终端与主站之间的主题、角色和消息用途。[引用1]"
	got := formatOverviewAnswer(input)
	want := "该知识库主要覆盖协议与数据规范。\n1. **MQTT 报文与 Topic 设计**：详细定义终端与主站之间的主题、角色和消息用途。[引用1]"
	if got != want {
		t.Fatalf("formatOverviewAnswer() = %q, want %q", got, want)
	}
}

func TestOverviewChunkScorePrefersMeaningfulSection(t *testing.T) {
	meaningful := overviewChunkScore("通信协议架构", strings.Repeat("内容", 200), 3)
	weak := overviewChunkScore("2.3", strings.Repeat("内容", 200), 0)
	if meaningful <= weak {
		t.Fatalf("meaningful section score %.2f must exceed weak section score %.2f", meaningful, weak)
	}
}

func TestOverviewSummaryUsesDocumentNameAndStructuredContext(t *testing.T) {
	summary := &overviewDocSummary{
		document: model.Document{Filename: "enterprise-protocol.docx"},
		sections: []string{"2.3", "通信架构", "消息格式"},
		chunk:    &types.RetrievalChunk{Content: "本文规定平台与终端之间的通信流程和消息结构。"},
	}
	if title := buildOverviewTitle(summary); title != "enterprise-protocol" {
		t.Fatalf("unexpected title: %s", title)
	}
	description := buildOverviewDescription(summary)
	if !strings.Contains(description, "通信架构、消息格式") || !strings.Contains(description, "通信流程") {
		t.Fatalf("unexpected description: %s", description)
	}
}

func TestReferencedSourcesKeepsOnlyUsedCitations(t *testing.T) {
	chunks, links := ReferencedSources("结论。[引用2][外链1]", []types.RetrievalChunk{{ID: "1"}, {ID: "2"}}, []types.ExternalLink{{URL: "https://example.com"}, {URL: "https://unused.example.com"}})
	if len(chunks) != 1 || chunks[0].ID != "2" {
		t.Fatalf("unexpected chunks: %#v", chunks)
	}
	if len(links) != 1 || links[0].URL != "https://example.com" {
		t.Fatalf("unexpected links: %#v", links)
	}
}

func TestRemapReferencedSourcesKeepsAnswerAndSourcesAligned(t *testing.T) {
	answer, chunks, links := RemapReferencedSources(
		"结论一。[引用4] 结论二。[引用2][外链3]",
		[]types.RetrievalChunk{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}},
		[]types.ExternalLink{{URL: "1"}, {URL: "2"}, {URL: "3"}},
	)
	if answer != "结论一。[引用1] 结论二。[引用2][外链1]" {
		t.Fatalf("unexpected remapped answer: %s", answer)
	}
	if len(chunks) != 2 || chunks[0].ID != "4" || chunks[1].ID != "2" || len(links) != 1 || links[0].URL != "3" {
		t.Fatalf("unexpected remapped sources: chunks=%#v links=%#v", chunks, links)
	}
}
