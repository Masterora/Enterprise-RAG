package parser

import "testing"

func TestParsePlainTextBuildsStructuredSections(t *testing.T) {
	text := "一、总览\n系统介绍\n\n（一）检索流程\n向量检索负责召回相关片段。\n\n二、删除流程\n先删除向量，再删除数据库记录。"

	plainText, metadata, err := parsePlainText([]byte(text))
	if err != nil {
		t.Fatalf("parsePlainText returned error: %v", err)
	}
	if plainText == "" {
		t.Fatal("plainText is empty")
	}
	if len(metadata) == 0 {
		t.Fatal("metadata is empty")
	}

	_, ok := detectPlainTextHeading("一、总览")
	if !ok {
		t.Fatal("expected level-1 heading to be recognized")
	}
	level, ok := detectPlainTextHeading("（一）检索流程")
	if !ok || level != 2 {
		t.Fatalf("expected level-2 heading, got level=%d ok=%t", level, ok)
	}
}
