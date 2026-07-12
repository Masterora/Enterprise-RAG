package parser

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"enterprise-rag/backend/internal/model"
)

func TestParsePlainText(t *testing.T) {
	t.Run("builds structured sections", func(t *testing.T) {
		text := "一、总览\n系统介绍\n\n（一）检索流程\n向量检索负责召回相关片段。\n\n二、删除流程\n先删除向量，再删除数据库记录。"

		plainText, metadata, err := parsePlainText([]byte(text))
		if err != nil {
			t.Fatalf("parsePlainText() error = %v", err)
		}

		segments := mustSegments(t, metadata)
		if plainText == "" {
			t.Fatal("plainText is empty")
		}
		if len(segments) != 3 {
			t.Fatalf("segment count = %d, want 3", len(segments))
		}
		if segments[0].Section != "一、总览" {
			t.Fatalf("first section = %q", segments[0].Section)
		}
		if segments[1].Section != "一、总览 / （一）检索流程" {
			t.Fatalf("second section = %q", segments[1].Section)
		}
		if segments[2].Section != "二、删除流程" {
			t.Fatalf("third section = %q", segments[2].Section)
		}
	})

	t.Run("keeps numbered list in same section", func(t *testing.T) {
		text := `三、在 Jaeger 中应该看什么
进入 Jaeger UI 后，最常看的内容包括：

1. Trace 列表：查看最近请求。
2. Duration：请求总耗时。
3. Spans：请求经过了哪些服务和步骤。`

		_, metadata, err := parsePlainText([]byte(text))
		if err != nil {
			t.Fatalf("parsePlainText() error = %v", err)
		}

		segments := mustSegments(t, metadata)
		if len(segments) != 1 {
			t.Fatalf("segment count = %d, want 1", len(segments))
		}

		if segments[0].Section != "三、在 Jaeger 中应该看什么" {
			t.Fatalf("section = %q", segments[0].Section)
		}
		if !strings.Contains(segments[0].Content, "1. Trace 列表") ||
			!strings.Contains(segments[0].Content, "3. Spans") {
			t.Fatalf("numbered list was split: %q", segments[0].Content)
		}
	})
}

func TestParseMarkdown(t *testing.T) {
	t.Run("builds nested heading path", func(t *testing.T) {
		text := "# 总览\n系统介绍\n\n## 检索流程\n向量检索负责召回相关片段。"

		plainText, metadata, err := parseMarkdown([]byte(text))
		if err != nil {
			t.Fatalf("parseMarkdown() error = %v", err)
		}

		segments := mustSegments(t, metadata)
		if plainText == "" {
			t.Fatal("plainText is empty")
		}
		if len(segments) != 2 {
			t.Fatalf("segment count = %d, want 2", len(segments))
		}
		if segments[0].Section != "总览" {
			t.Fatalf("segment[0].Section = %q", segments[0].Section)
		}
		if segments[1].Section != "总览 / 检索流程" {
			t.Fatalf("segment[1].Section = %q", segments[1].Section)
		}
	})

	t.Run("does not treat fenced code heading as real heading", func(t *testing.T) {
		text := "## 示例\n```md\n# 这不是标题\n```\n解释文本"

		_, metadata, err := parseMarkdown([]byte(text))
		if err != nil {
			t.Fatalf("parseMarkdown() error = %v", err)
		}

		segments := mustSegments(t, metadata)
		if len(segments) != 1 {
			t.Fatalf("segment count = %d, want 1", len(segments))
		}
		if segments[0].Section != "示例" {
			t.Fatalf("segment[0].Section = %q", segments[0].Section)
		}
		if !strings.Contains(segments[0].Content, "# 这不是标题") {
			t.Fatalf("code block content missing: %q", segments[0].Content)
		}
	})
}

func TestDetectPlainTextHeading(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		level int
		ok    bool
	}{
		{name: "chapter heading", line: "一、总览", level: 1, ok: true},
		{name: "nested heading", line: "（一）检索流程", level: 2, ok: true},
		{name: "question heading", line: "Q1：为什么直接选择 Milvus", level: 2, ok: true},
		{name: "long paragraph", line: strings.Repeat("长", 81), level: 0, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, ok := detectPlainTextHeading(tt.line)
			if ok != tt.ok || level != tt.level {
				t.Fatalf("detectPlainTextHeading(%q) = (%d, %t), want (%d, %t)", tt.line, level, ok, tt.level, tt.ok)
			}
		})
	}
}

func TestParseDOCX(t *testing.T) {
	data := buildDOCXFile(t,
		docxParagraph("文档总览", "Heading1", false),
		docxParagraph("这是第一段内容。", "", false),
		docxParagraph("检索流程", "Heading2", false),
		docxParagraph("向量检索负责召回相关片段。", "", false),
		docxParagraph("Trace 列表", "", true),
		docxParagraph("2. Duration", "", true),
		docxTable(
			[]string{"字段", "说明"},
			[]string{"doc_id", "文档唯一标识"},
		),
	)

	plainText, metadata, err := parseDOCX(data, ParseOptions{})
	if err != nil {
		t.Fatalf("parseDOCX() error = %v", err)
	}

	segments := mustSegments(t, metadata)
	if len(segments) != 2 {
		t.Fatalf("segment count = %d, want 2", len(segments))
	}
	if segments[0].Section != "文档总览" {
		t.Fatalf("segment[0].Section = %q", segments[0].Section)
	}
	if segments[1].Section != "文档总览 / 检索流程" {
		t.Fatalf("segment[1].Section = %q", segments[1].Section)
	}
	if !strings.Contains(segments[1].Content, "- Trace 列表") {
		t.Fatalf("list item missing from paragraph segment: %q", segments[1].Content)
	}
	if !strings.Contains(segments[1].Content, "字段 | 说明") ||
		!strings.Contains(segments[1].Content, "doc_id | 文档唯一标识") {
		t.Fatalf("table content missing: %q", segments[1].Content)
	}
	if !strings.Contains(plainText, "向量检索负责召回相关片段。") {
		t.Fatalf("plainText missing expected paragraph: %q", plainText)
	}
}

func TestParsePPTX(t *testing.T) {
	data := buildPPTXFile(t,
		pptxSlide(
			"系统总览",
			[]string{
				"配电物联网平台通过 MQTT 与终端交互。",
				"支持主题订阅、状态上报与指令响应。",
			},
			[][]string{
				{"字段", "说明"},
				{"topic", "主题"},
			},
		),
		pptxSlide(
			"检索流程",
			[]string{
				"向量检索负责召回相关片段。",
			},
			nil,
		),
	)

	plainText, metadata, err := parsePPTX(data, ParseOptions{})
	if err != nil {
		t.Fatalf("parsePPTX() error = %v", err)
	}

	segments := mustSegments(t, metadata)
	if len(segments) != 2 {
		t.Fatalf("segment count = %d, want 2", len(segments))
	}
	if segments[0].Section != "第1页 / 系统总览" {
		t.Fatalf("segment[0].Section = %q", segments[0].Section)
	}
	if !strings.Contains(segments[0].Content, "配电物联网平台通过 MQTT 与终端交互。") {
		t.Fatalf("segment[0].Content missing paragraph: %q", segments[0].Content)
	}
	if !strings.Contains(segments[0].Content, "字段 | 说明") || !strings.Contains(segments[0].Content, "topic | 主题") {
		t.Fatalf("segment[0].Content missing table: %q", segments[0].Content)
	}
	if segments[1].Section != "第2页 / 检索流程" {
		t.Fatalf("segment[1].Section = %q", segments[1].Section)
	}
	if !strings.Contains(plainText, "向量检索负责召回相关片段。") {
		t.Fatalf("plainText missing expected content: %q", plainText)
	}
}

func TestParseDOCViaTextutil(t *testing.T) {
	if _, err := exec.LookPath("textutil"); err != nil {
		t.Skip("textutil not available")
	}

	plainText, metadata, err := parseDOC([]byte("一、总览\n这是 doc 内容。\n\n二、流程\n先解析，再切块。"), ParseOptions{})
	if err != nil {
		t.Fatalf("parseDOC() error = %v", err)
	}

	segments := mustSegments(t, metadata)
	if len(segments) == 0 {
		t.Fatal("segment count is 0")
	}
	if !strings.Contains(plainText, "这是 doc 内容。") {
		t.Fatalf("plainText missing expected content: %q", plainText)
	}
}

func TestAppendPDFSegments(t *testing.T) {
	pageText := strings.Join([]string{
		"一、Jaeger 总览",
		"",
		"Jaeger 用于展示调用链路和耗时。",
		"",
		"字段    说明",
		"trace_id    调用链唯一标识",
		"duration    总耗时",
		"",
		"二、排查重点",
		"",
		"1. Trace 列表",
		"2. Duration",
	}, "\n")

	var (
		plainText strings.Builder
		segments  []model.ParseSegment
	)
	appendPDFSegments(&plainText, &segments, 1, pageText)

	if len(segments) != 2 {
		t.Fatalf("segment count = %d, want 2", len(segments))
	}
	if segments[0].Page != 1 || segments[0].Section != "一、Jaeger 总览" {
		t.Fatalf("segment[0] = %#v", segments[0])
	}
	if !strings.Contains(segments[0].Content, "字段 | 说明") || !strings.Contains(segments[0].Content, "duration | 总耗时") {
		t.Fatalf("segment[0].Content missing normalized table: %q", segments[0].Content)
	}
	if segments[1].Section != "二、排查重点" {
		t.Fatalf("segment[1].Section = %q", segments[1].Section)
	}
	if !strings.Contains(segments[1].Content, "1. Trace 列表") || !strings.Contains(segments[1].Content, "2. Duration") {
		t.Fatalf("segment[1].Content missing list items: %q", segments[1].Content)
	}
	if !strings.Contains(plainText.String(), "Jaeger 用于展示调用链路和耗时。") {
		t.Fatalf("plainText missing expected content: %q", plainText.String())
	}
}

func TestNormalizePDFParagraph(t *testing.T) {
	text := strings.Join([]string{
		"Jaeger is an open source",
		"distributed tracing platform",
		"used for monitoring.",
		"",
		"用于展示调用链路",
		"和耗时。",
		"",
		"1. Trace 列表",
		"2. Duration",
	}, "\n")

	got := normalizePDFParagraph(text)
	if !strings.Contains(got, "Jaeger is an open source distributed tracing platform used for monitoring.") {
		t.Fatalf("english paragraph not joined: %q", got)
	}
	if !strings.Contains(got, "用于展示调用链路和耗时。") {
		t.Fatalf("chinese paragraph not joined: %q", got)
	}
	if strings.Contains(got, "1. Trace 列表 2. Duration") {
		t.Fatalf("list lines were incorrectly merged: %q", got)
	}
}

func TestDOCXHeadingLevel(t *testing.T) {
	tests := []struct {
		name     string
		style    string
		content  string
		isList   bool
		want     int
		wantBool bool
	}{
		{name: "heading style", style: "Heading2", content: "检索流程", want: 2, wantBool: true},
		{name: "list is not heading", style: "", content: "1. 列表项", isList: true, want: 0, wantBool: false},
		{name: "fallback plain text heading", style: "", content: "一、总览", want: 1, wantBool: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := docxHeadingLevel(tt.style, tt.content, tt.isList)
			if got != tt.want || ok != tt.wantBool {
				t.Fatalf("docxHeadingLevel(%q, %q, %t) = (%d, %t), want (%d, %t)", tt.style, tt.content, tt.isList, got, ok, tt.want, tt.wantBool)
			}
		})
	}
}

func mustSegments(t *testing.T, metadata []byte) []model.ParseSegment {
	t.Helper()

	var parsed model.DocumentMetadata
	if err := json.Unmarshal(metadata, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return parsed.Segments
}

type docxParagraphSpec struct {
	text   string
	style  string
	isList bool
}

type docxTableSpec struct {
	rows [][]string
}

type pptxSlideSpec struct {
	title string
	body  []string
	table [][]string
}

func docxParagraph(text, style string, isList bool) docxParagraphSpec {
	return docxParagraphSpec{text: text, style: style, isList: isList}
}

func docxTable(rows ...[]string) docxTableSpec {
	return docxTableSpec{rows: rows}
}

func pptxSlide(title string, body []string, table [][]string) pptxSlideSpec {
	return pptxSlideSpec{title: title, body: body, table: table}
}

func buildDOCXFile(t *testing.T, blocks ...any) []byte {
	t.Helper()

	var document strings.Builder
	document.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	document.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)

	for _, block := range blocks {
		switch value := block.(type) {
		case docxParagraphSpec:
			document.WriteString(`<w:p>`)
			if value.style != "" || value.isList {
				document.WriteString(`<w:pPr>`)
				if value.style != "" {
					document.WriteString(`<w:pStyle w:val="` + xmlEscape(value.style) + `"/>`)
				}
				if value.isList {
					document.WriteString(`<w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr>`)
				}
				document.WriteString(`</w:pPr>`)
			}
			document.WriteString(`<w:r><w:t>`)
			document.WriteString(xmlEscape(value.text))
			document.WriteString(`</w:t></w:r></w:p>`)
		case docxTableSpec:
			document.WriteString(`<w:tbl>`)
			for _, row := range value.rows {
				document.WriteString(`<w:tr>`)
				for _, cell := range row {
					document.WriteString(`<w:tc><w:p><w:r><w:t>`)
					document.WriteString(xmlEscape(cell))
					document.WriteString(`</w:t></w:r></w:p></w:tc>`)
				}
				document.WriteString(`</w:tr>`)
			}
			document.WriteString(`</w:tbl>`)
		default:
			t.Fatalf("unsupported docx block type %T", block)
		}
	}

	document.WriteString(`</w:body></w:document>`)

	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	file, err := archive.Create("word/document.xml")
	if err != nil {
		t.Fatalf("Create(document.xml) error = %v", err)
	}
	if _, err := file.Write([]byte(document.String())); err != nil {
		t.Fatalf("Write(document.xml) error = %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buf.Bytes()
}

func buildPPTXFile(t *testing.T, slides ...pptxSlideSpec) []byte {
	t.Helper()

	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for index, slide := range slides {
		file, err := archive.Create("ppt/slides/slide" + strconv.Itoa(index+1) + ".xml")
		if err != nil {
			t.Fatalf("Create(slide%d.xml) error = %v", index+1, err)
		}
		var document strings.Builder
		document.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
		document.WriteString(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree>`)
		if slide.title != "" {
			document.WriteString(`<p:sp><p:txBody><a:p><a:r><a:t>`)
			document.WriteString(xmlEscape(slide.title))
			document.WriteString(`</a:t></a:r></a:p></p:txBody></p:sp>`)
		}
		for _, paragraph := range slide.body {
			document.WriteString(`<p:sp><p:txBody><a:p><a:r><a:t>`)
			document.WriteString(xmlEscape(paragraph))
			document.WriteString(`</a:t></a:r></a:p></p:txBody></p:sp>`)
		}
		if len(slide.table) > 0 {
			document.WriteString(`<p:graphicFrame><a:graphic><a:graphicData><a:tbl>`)
			for _, row := range slide.table {
				document.WriteString(`<a:tr>`)
				for _, cell := range row {
					document.WriteString(`<a:tc><a:txBody><a:p><a:r><a:t>`)
					document.WriteString(xmlEscape(cell))
					document.WriteString(`</a:t></a:r></a:p></a:txBody></a:tc>`)
				}
				document.WriteString(`</a:tr>`)
			}
			document.WriteString(`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`)
		}
		document.WriteString(`</p:spTree></p:cSld></p:sld>`)
		if _, err := file.Write([]byte(document.String())); err != nil {
			t.Fatalf("Write(slide%d.xml) error = %v", index+1, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buf.Bytes()
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}
