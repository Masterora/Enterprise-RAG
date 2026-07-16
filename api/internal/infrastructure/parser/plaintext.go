package parser

import (
	"regexp"
	"strings"

	"enterprise-rag/api/internal/model"
)

func parsePlainText(data []byte) (string, []byte, error) {
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	rawSegments := strings.Split(normalized, "\n\n")

	var (
		plainText strings.Builder
		segments  []model.ParseSegment
		headings  []string
	)

	for _, raw := range rawSegments {
		block := normalizeTextBlock(raw)
		if block == "" {
			continue
		}

		lines := strings.Split(block, "\n")
		firstLine := strings.TrimSpace(lines[0])
		if level, ok := detectPlainTextHeading(firstLine); ok && !isPlainTextListBlock(lines) {
			headings = trimHeadingPath(headings, level)
			headings = append(headings, cleanHeadingText(firstLine))

			remainder := normalizeTextBlock(strings.Join(lines[1:], "\n"))
			if remainder == "" {
				continue
			}

			appendParseSegment(&plainText, &segments, strings.Join(headings, " / "), remainder)
			continue
		}

		appendParseSegment(&plainText, &segments, currentSection(headings), block)
	}

	if len(segments) == 0 {
		content := strings.TrimSpace(normalized)
		appendParseSegment(&plainText, &segments, "text", content)
	}

	return finalizeParseResult(plainText.String(), segments)
}

func isPlainTextListBlock(lines []string) bool {
	if len(lines) < 2 {
		return false
	}
	return plainTextNumberedItemPattern.MatchString(strings.TrimSpace(lines[0])) &&
		plainTextNumberedItemPattern.MatchString(strings.TrimSpace(lines[1]))
}

func currentSection(headings []string) string {
	if len(headings) == 0 {
		return "text"
	}
	return strings.Join(headings, " / ")
}

func trimHeadingPath(headings []string, level int) []string {
	if level <= 1 {
		return headings[:0]
	}
	if level-1 < len(headings) {
		return headings[:level-1]
	}
	return headings
}

func cleanHeadingText(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, ":")
	line = strings.TrimSuffix(line, "：")
	return strings.TrimSpace(line)
}

var (
	plainTextNumberedItemPattern = regexp.MustCompile(`^\d+[、.．]\s*.+$`)
	plainTextListMarkerPattern   = regexp.MustCompile(`^([*-]|\d+[、.．)]|[（(]\d+[)）]|[一二三四五六七八九十]+[、.．])\s*.+$`)
	plainTextHeadingPatterns     = []struct {
		regex *regexp.Regexp
		level int
	}{
		{regexp.MustCompile(`^第[一二三四五六七八九十百千万0-9]+[章节部分篇].*$`), 1},
		{regexp.MustCompile(`^[一二三四五六七八九十百千万]+[、.．]\s*.+$`), 1},
		{plainTextNumberedItemPattern, 1},
		{regexp.MustCompile(`^\d+[)）]\s*.+$`), 1},
		{regexp.MustCompile(`^[\(（][一二三四五六七八九十百千万0-9]+[\)）]\s*.+$`), 2},
		{regexp.MustCompile(`^\d+\.\d+(?:\.\d+)*\s*.+$`), 2},
		{regexp.MustCompile(`^\d+\.\d+(?:\.\d+)*[)）]\s*.+$`), 2},
		{regexp.MustCompile(`^(Q|q)\d+[:：]\s*.+$`), 2},
		{regexp.MustCompile(`^【[^】]{1,40}】$`), 2},
	}
)

func detectPlainTextHeading(line string) (int, bool) {
	line = strings.TrimSpace(line)
	if line == "" || len([]rune(line)) > 80 || strings.Contains(line, "\n") {
		return 0, false
	}
	for _, pattern := range plainTextHeadingPatterns {
		if pattern.regex.MatchString(line) {
			return pattern.level, true
		}
	}
	return 0, false
}
