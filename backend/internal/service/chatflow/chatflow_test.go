package chatflow

import (
	"strings"
	"testing"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/types"
)

func TestBuildPromptIncludesGroundingRules(t *testing.T) {
	prompt := BuildPrompt(config.PromptConf{}, "Jaeger", []types.RetrievalChunk{{
		DocName: "05_trace_jaeger_observability.txt",
		Section: "三、在 Jaeger 中应该看什么",
		Content: "最常看的内容包括 Trace 列表、Duration、Spans、Service、Tags、Logs。",
	}}, []types.ExternalLink{{
		Title:   "Jaeger overview",
		URL:     "https://example.com/jaeger",
		Snippet: "Jaeger is an open source distributed tracing system.",
	}}, true)

	for _, expected := range []string{
		"正式、完整、信息充分",
		"忽略仅是目录、测试题、清单、术语表之类的弱相关内容",
		"[外链1]",
		"[引用1]",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing expected text %q\n%s", expected, prompt)
		}
	}
}

func TestParseRouteResponse(t *testing.T) {
	tests := []struct {
		response string
		want     QueryAnalysis
		ok       bool
	}{
		{response: `{"route":"overview","search_query":""}`, want: QueryAnalysis{Route: QueryRouteOverview}, ok: true},
		{response: "```json\n{\"route\":\"navigation\",\"search_query\":\"\"}\n```", want: QueryAnalysis{Route: QueryRouteNavigation}, ok: true},
		{response: `{"route":"unknown"}`, ok: false},
		{response: "overview", ok: false},
	}

	for _, tt := range tests {
		got, ok := parseRouteResponse(tt.response)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("parseRouteResponse(%q) = (%q, %v), want (%q, %v)", tt.response, got, ok, tt.want, tt.ok)
		}
	}
}

func TestBuildRoutePromptUsesCustomTemplate(t *testing.T) {
	prompt := BuildRoutePrompt(config.PromptConf{RouteTemplate: "判断={{question}}"}, "能干什么")
	if prompt != "判断=能干什么" {
		t.Fatalf("unexpected route prompt: %q", prompt)
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

func TestBuildOverviewLeadIncludesReferenceSuffix(t *testing.T) {
	got := buildOverviewLead("ut", 4, []string{"MQTT 通信协议", "E语言规范"}, []types.RetrievalChunk{{}, {}, {}})
	want := "知识库“ut”当前共包含 4 篇已索引文档，主要覆盖 MQTT 通信协议、E语言规范 等方向。 [引用1] [引用2] [引用3]"
	if got != want {
		t.Fatalf("buildOverviewLead() = %q, want %q", got, want)
	}
}

func TestBuildPromptUsesCustomTemplate(t *testing.T) {
	prompt := BuildPrompt(config.PromptConf{
		AnswerTemplate: "问题={{question}}\n资料={{knowledge_chunks}}",
	}, "测试问题", []types.RetrievalChunk{{
		DocName: "a.md",
		Section: "概述",
		Content: "测试内容",
	}}, nil, false)

	for _, expected := range []string{"问题=测试问题", "资料=[引用1]", "文档：a.md", "内容：测试内容"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("custom prompt missing %q\n%s", expected, prompt)
		}
	}
}

func TestBuildPromptUsesExplanationTemplate(t *testing.T) {
	prompt := BuildPrompt(config.PromptConf{ExplanationTemplate: "解释={{question}}"}, "为什么先删除向量？", nil, nil, false)
	if prompt != "解释=为什么先删除向量？" {
		t.Fatalf("unexpected prompt: %q", prompt)
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
