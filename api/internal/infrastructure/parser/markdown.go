package parser

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"

	"enterprise-rag/api/internal/model"
)

func parseMarkdown(data []byte) (string, []byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var (
		plainText strings.Builder
		buffer    strings.Builder
		segments  []model.ParseSegment
		headings  []string
		inCode    bool
	)

	flush := func() {
		content := normalizeMarkdownBlock(buffer.String())
		if content == "" {
			return
		}
		appendParseSegment(&plainText, &segments, currentSection(headings), content)
		buffer.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if markdownFencePattern.MatchString(trimmed) {
			inCode = !inCode
			if buffer.Len() > 0 {
				buffer.WriteByte('\n')
			}
			buffer.WriteString(line)
			continue
		}
		if !inCode {
			if level, title, ok := detectMarkdownHeading(trimmed); ok {
				flush()
				headings = trimHeadingPath(headings, level)
				headings = append(headings, title)
				continue
			}
			if isMarkdownFrontMatterBoundary(trimmed) && plainText.Len() == 0 && len(segments) == 0 && buffer.Len() == 0 {
				continue
			}
		}
		if strings.HasPrefix(trimmed, "---") && plainText.Len() == 0 && len(segments) == 0 && buffer.Len() == 0 {
			flush()
			continue
		}
		if buffer.Len() > 0 {
			buffer.WriteByte('\n')
		}
		buffer.WriteString(line)
	}
	flush()
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	if len(segments) == 0 {
		appendParseSegment(&plainText, &segments, "text", normalizeMarkdownBlock(string(data)))
	}

	return finalizeParseResult(plainText.String(), segments)
}

var markdownFencePattern = regexp.MustCompile("^(```|~~~)")

func detectMarkdownHeading(line string) (int, string, bool) {
	if line == "" {
		return 0, "", false
	}
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	if len(line) > level && line[level] != ' ' && line[level] != '\t' {
		return 0, "", false
	}
	title := cleanHeadingText(strings.TrimSpace(line[level:]))
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

func normalizeMarkdownBlock(block string) string {
	block = strings.ReplaceAll(block, "\r\n", "\n")
	block = strings.ReplaceAll(block, "\r", "\n")
	lines := strings.Split(block, "\n")
	cleaned := make([]string, 0, len(lines))
	inCode := false
	lastBlank := false
	for _, line := range lines {
		trimmedRight := strings.TrimRight(line, " \t")
		trimmed := strings.TrimSpace(trimmedRight)
		if markdownFencePattern.MatchString(trimmed) {
			inCode = !inCode
			cleaned = append(cleaned, trimmedRight)
			lastBlank = false
			continue
		}
		if inCode {
			cleaned = append(cleaned, trimmedRight)
			lastBlank = false
			continue
		}
		if trimmed == "" {
			if len(cleaned) == 0 || lastBlank {
				continue
			}
			cleaned = append(cleaned, "")
			lastBlank = true
			continue
		}
		cleaned = append(cleaned, trimmedRight)
		lastBlank = false
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func isMarkdownFrontMatterBoundary(line string) bool {
	return line == "---"
}
