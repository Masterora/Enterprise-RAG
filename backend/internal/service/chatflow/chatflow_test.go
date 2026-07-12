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

func TestRouteQuery(t *testing.T) {
	tests := []struct {
		query string
		want  QueryRoute
	}{
		{query: "这个知识库有什么内容", want: QueryRouteOverview},
		{query: "这个库能解决什么问题", want: QueryRouteOverview},
		{query: "有哪些文档讲了 Milvus", want: QueryRouteNavigation},
		{query: "这个知识库里哪些文档讲 MQTT", want: QueryRouteNavigation},
		{query: "删除文档时为什么要先删除 Milvus 里的向量", want: QueryRouteRAG},
		{query: "Jaeger 主要用来看什么", want: QueryRouteRAG},
	}

	for _, tt := range tests {
		if got := RouteQuery(tt.query); got != tt.want {
			t.Fatalf("RouteQuery(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}

func TestExtractNavigationTopic(t *testing.T) {
	if got := extractNavigationTopic("有哪些文档讲了 Milvus"); got != "Milvus" {
		t.Fatalf("extractNavigationTopic() = %q, want %q", got, "Milvus")
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
	got := buildOverviewLead("这个知识库有什么内容", "ut", 4, []string{"MQTT 通信协议", "E语言规范"}, []types.RetrievalChunk{{}, {}, {}})
	want := "知识库“ut”当前共包含 4 篇已索引文档，主要覆盖 MQTT 通信协议、E语言规范 等方向。 [引用1] [引用2] [引用3]"
	if got != want {
		t.Fatalf("buildOverviewLead() = %q, want %q", got, want)
	}
}

func TestBuildOverviewLeadSupportsUseCaseQuestion(t *testing.T) {
	got := buildOverviewLead("这个库能解决什么问题", "ut", 4, []string{"MQTT 通信协议", "E语言规范"}, []types.RetrievalChunk{{}})
	if !strings.Contains(got, "主要可用于支撑 MQTT 通信协议、E语言规范 相关的协议理解、方案实现、联调排障与规范查阅。") {
		t.Fatalf("unexpected use-case lead: %q", got)
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
