package parser

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/task"

	pdf "github.com/ledongthuc/pdf"
)

func parsePDF(data []byte, options ParseOptions) (string, []byte, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "encryption version") ||
			strings.Contains(strings.ToLower(err.Error()), "/filter /standard") {
			return "", nil, errors.New("该 PDF 启用了加密或权限限制，请先移除密码或另存为未加密 PDF 后再上传")
		}
		return "", nil, err
	}

	segments := make([]model.ParseSegment, 0, reader.NumPage()*2)
	var plainText strings.Builder
	extractedPages := 0
	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			return "", nil, err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		extractedPages++

		if options.ProcessingMode == task.ProcessingModeStandard {
			appendPDFSegment(&plainText, &segments, pageNum, fmt.Sprintf("第%d页", pageNum), normalizePDFParagraph(text))
			continue
		}
		appendPDFSegments(&plainText, &segments, pageNum, text)
	}

	if extractedPages == 0 && options.ProcessingMode == task.ProcessingModeEnhanced {
		return "", nil, errors.New("文档未提取到可读文字，疑似扫描件或图片型 PDF，当前未启用 OCR")
	}

	return finalizeParseResult(plainText.String(), segments)
}

func appendPDFSegments(plainText *strings.Builder, segments *[]model.ParseSegment, page int, text string) {
	blocks := splitPDFBlocks(text)
	if len(blocks) == 0 {
		blocks = []string{normalizePDFParagraph(text)}
	}

	headings := make([]string, 0)
	for _, block := range blocks {
		lines := splitPDFLines(block)
		if isPDFTableBlock(lines) {
			block = normalizePDFTableBlock(lines)
		} else {
			block = normalizePDFParagraph(block)
			lines = strings.Split(block, "\n")
		}
		if block == "" {
			continue
		}

		firstLine := strings.TrimSpace(lines[0])
		if level, ok := detectPDFHeading(firstLine, lines); ok {
			headings = trimHeadingPath(headings, level)
			headings = append(headings, cleanHeadingText(firstLine))

			remainder := normalizePDFParagraph(strings.Join(lines[1:], "\n"))
			if remainder == "" {
				continue
			}

			appendPDFSegment(plainText, segments, page, currentSection(headings), remainder)
			continue
		}

		section := currentSection(headings)
		appendPDFSegment(plainText, segments, page, section, block)
	}
}

func appendPDFSegment(plainText *strings.Builder, segments *[]model.ParseSegment, page int, section, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	if strings.TrimSpace(section) == "" {
		section = "page"
	}
	if len(*segments) > 0 {
		last := &(*segments)[len(*segments)-1]
		if last.Page == page && last.Section == section {
			last.Content += "\n\n" + content
		} else {
			*segments = append(*segments, model.ParseSegment{
				Page:    page,
				Section: section,
				Content: content,
			})
		}
	} else {
		*segments = append(*segments, model.ParseSegment{
			Page:    page,
			Section: section,
			Content: content,
		})
	}
	if plainText.Len() > 0 {
		plainText.WriteString("\n\n")
	}
	plainText.WriteString(content)
}

func splitPDFBlocks(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	rawBlocks := regexp.MustCompile(`\n\s*\n+`).Split(normalized, -1)
	blocks := make([]string, 0, len(rawBlocks))
	for _, block := range rawBlocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		blocks = append(blocks, block)
	}
	return blocks
}

func detectPDFHeading(firstLine string, lines []string) (int, bool) {
	if level, ok := detectPlainTextHeading(firstLine); ok && !isPlainTextListBlock(lines) {
		return level, true
	}

	firstLine = strings.TrimSpace(firstLine)
	if firstLine == "" || len(lines) > 2 {
		return 0, false
	}
	runes := []rune(firstLine)
	if len(runes) > 28 || len(runes) < 2 {
		return 0, false
	}
	if strings.ContainsAny(firstLine, "。；;，,！？?!.:：") {
		return 0, false
	}
	if pdfLikelyWrappedEnglishLinePattern.MatchString(firstLine) {
		return 0, false
	}
	return 1, true
}

func splitPDFLines(block string) []string {
	lines := strings.Split(block, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		result = append(result, line)
	}
	return result
}

func isPDFTableBlock(lines []string) bool {
	if len(lines) < 2 {
		return false
	}
	matchedRows := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "|") || strings.Contains(line, "\t") || pdfTableGapPattern.MatchString(line) {
			matchedRows++
		}
	}
	return matchedRows >= 2
}

func normalizePDFParagraph(block string) string {
	lines := splitPDFLines(block)
	if len(lines) == 0 {
		return ""
	}
	paragraphs := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(paragraphs) == 0 {
			paragraphs = append(paragraphs, line)
			continue
		}
		lastIndex := len(paragraphs) - 1
		if shouldJoinPDFLine(paragraphs[lastIndex], line) {
			paragraphs[lastIndex] = joinPDFLine(paragraphs[lastIndex], line)
			continue
		}
		paragraphs = append(paragraphs, line)
	}
	return strings.TrimSpace(strings.Join(paragraphs, "\n"))
}

func shouldJoinPDFLine(previous, current string) bool {
	if previous == "" || current == "" {
		return false
	}
	if detectPDFListLine(current) || detectPDFListLine(previous) {
		return false
	}
	if _, ok := detectPDFHeading(current, []string{current}); ok {
		return false
	}
	if isPDFTableBlock([]string{previous, current}) {
		return false
	}
	lastRune := []rune(previous)[len([]rune(previous))-1]
	if strings.ContainsRune("。！？；;:：", lastRune) {
		return false
	}
	if lastRune == '-' || lastRune == '‑' || lastRune == '–' {
		return true
	}
	return true
}

func joinPDFLine(previous, current string) string {
	if strings.HasSuffix(previous, "-") || strings.HasSuffix(previous, "‑") || strings.HasSuffix(previous, "–") {
		return strings.TrimRight(previous[:len(previous)-1], " \t") + current
	}
	if endsWithASCIIWord(previous) && startsWithASCIIWord(current) {
		return previous + " " + current
	}
	if endsWithCJK(previous) || startsWithCJK(current) {
		return previous + current
	}
	return previous + " " + current
}

func detectPDFListLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	return pdfListMarkerPattern.MatchString(line)
}

func endsWithASCIIWord(text string) bool {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return false
	}
	last := runes[len(runes)-1]
	return (last >= 'a' && last <= 'z') || (last >= 'A' && last <= 'Z') || (last >= '0' && last <= '9')
}

func startsWithASCIIWord(text string) bool {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return false
	}
	first := runes[0]
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || (first >= '0' && first <= '9')
}

func endsWithCJK(text string) bool {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return false
	}
	return isCJKRune(runes[len(runes)-1])
}

func startsWithCJK(text string) bool {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return false
	}
	return isCJKRune(runes[0])
}

func isCJKRune(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF)
}

func normalizePDFTableBlock(lines []string) string {
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.Contains(line, "|"):
			rows = append(rows, line)
		case strings.Contains(line, "\t"):
			cells := trimPDFCells(strings.Split(line, "\t"))
			rows = append(rows, strings.Join(cells, " | "))
		case pdfTableGapPattern.MatchString(line):
			cells := pdfTableGapPattern.Split(line, -1)
			rows = append(rows, strings.Join(trimPDFCells(cells), " | "))
		default:
			rows = append(rows, line)
		}
	}
	return strings.Join(rows, "\n")
}

var pdfTableGapPattern = regexp.MustCompile(`\s{2,}`)
var pdfListMarkerPattern = regexp.MustCompile(`^([*-]|\d+[.、．)]|[（(]\d+[)）]|[一二三四五六七八九十]+[、.．])\s*.+$`)
var pdfLikelyWrappedEnglishLinePattern = regexp.MustCompile(`^[A-Za-z0-9]+(?:\s+[a-z][A-Za-z0-9/-]*){2,}$`)

func trimPDFCells(parts []string) []string {
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cells = append(cells, part)
	}
	return cells
}
