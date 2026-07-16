package knowledge

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	boilerplateHeadingPattern = regexp.MustCompile(`^(核心结论|补充说明(?:如下)?|说明如下|总结|结论)[：:]?$`)
	brokenNumberedLinePattern = regexp.MustCompile(`(^|\n)(\d+[.．、\)）])\s*\n+([^\n]+)`)
	citationLinePattern       = regexp.MustCompile(`\n+[ \t]*(\[(?:引用|外链)?\d+][^\n]*)`)
)

func normalizeAnswerText(answer string) string {
	lines := strings.Split(strings.ReplaceAll(answer, "\r\n", "\n"), "\n")
	normalized := make([]string, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			if len(normalized) > 0 && normalized[len(normalized)-1] != "" {
				normalized = append(normalized, "")
			}
			continue
		}
		if len(normalized) > 0 && numberedLineOnly(normalized[len(normalized)-1]) {
			normalized[len(normalized)-1] += " " + line
			continue
		}
		if boilerplateHeadingPattern.MatchString(line) {
			continue
		}
		normalized = append(normalized, line)
	}
	text := strings.Join(normalized, "\n")
	text = brokenNumberedLinePattern.ReplaceAllString(text, "$1$2 $3")
	text = citationLinePattern.ReplaceAllString(text, " $1")
	text = strings.ReplaceAll(text, "：\n", "：")
	text = strings.ReplaceAll(text, ":\n", ":")
	for strings.Contains(text, "\n\n") {
		text = strings.ReplaceAll(text, "\n\n", "\n")
	}
	return strings.TrimSpace(text)
}

func numberedLineOnly(line string) bool {
	runes := []rune(strings.TrimSpace(line))
	index := 0
	for index < len(runes) && unicode.IsDigit(runes[index]) {
		index++
	}
	if index == 0 || index >= len(runes) {
		return false
	}
	switch runes[index] {
	case '.', '．', '、', ')', '）':
		index++
	default:
		return false
	}
	for index < len(runes) {
		if !unicode.IsSpace(runes[index]) {
			return false
		}
		index++
	}
	return true
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func isWeakSection(section string) bool {
	section = strings.TrimSpace(strings.ToLower(section))
	if section == "" || section == "text" || section == "page" || strings.Contains(section, "目录") || strings.Contains(section, "测试问题") {
		return true
	}
	trimmed := strings.Trim(section, "第0123456789.．-_ /\\页page")
	return trimmed == ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
