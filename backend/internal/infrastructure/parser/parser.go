package parser

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"

	"enterprise-rag/backend/internal/model"

	pdf "github.com/ledongthuc/pdf"
	"github.com/minio/minio-go/v7"
)

func Parse(ctx context.Context, client *minio.Client, bucket, objectName, fileType string) (string, []byte, error) {
	object, err := client.GetObject(ctx, bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return "", nil, err
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return "", nil, err
	}

	switch strings.ToLower(fileType) {
	case "md", "markdown":
		return parseMarkdown(data)
	case "txt":
		return parsePlainText(data)
	case "pdf":
		return parsePDF(data)
	default:
		return "", nil, errors.New("unsupported document type")
	}
}

func parseMarkdown(data []byte) (string, []byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var (
		plainText strings.Builder
		section   string
		buffer    strings.Builder
		segments  []model.ParseSegment
	)

	flush := func() {
		content := strings.TrimSpace(buffer.String())
		if content == "" {
			return
		}
		segments = append(segments, model.ParseSegment{
			Page:    0,
			Section: section,
			Content: content,
		})
		if plainText.Len() > 0 {
			plainText.WriteString("\n\n")
		}
		plainText.WriteString(content)
		buffer.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			flush()
			section = strings.TrimSpace(strings.TrimLeft(line, "#"))
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
		content := strings.TrimSpace(string(data))
		segments = append(segments, model.ParseSegment{Content: content})
		plainText.WriteString(content)
	}

	metadata, err := json.Marshal(model.DocumentMetadata{Segments: segments})
	if err != nil {
		return "", nil, err
	}
	return plainText.String(), metadata, nil
}

func parsePDF(data []byte) (string, []byte, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, err
	}

	segments := make([]model.ParseSegment, 0, reader.NumPage())
	var plainText strings.Builder
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

		if plainText.Len() > 0 {
			plainText.WriteString("\n\n")
		}
		plainText.WriteString(text)
		segments = append(segments, model.ParseSegment{
			Page:    pageNum,
			Section: "page",
			Content: text,
		})
	}

	metadata, err := json.Marshal(model.DocumentMetadata{Segments: segments})
	if err != nil {
		return "", nil, err
	}
	return plainText.String(), metadata, nil
}

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
		if level, ok := detectPlainTextHeading(firstLine); ok {
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

	metadata, err := json.Marshal(model.DocumentMetadata{Segments: segments})
	if err != nil {
		return "", nil, err
	}
	return plainText.String(), metadata, nil
}

func appendParseSegment(plainText *strings.Builder, segments *[]model.ParseSegment, section, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	if strings.TrimSpace(section) == "" {
		section = "text"
	}
	*segments = append(*segments, model.ParseSegment{
		Page:    0,
		Section: section,
		Content: content,
	})
	if plainText.Len() > 0 {
		plainText.WriteString("\n\n")
	}
	plainText.WriteString(content)
}

func normalizeTextBlock(block string) string {
	lines := strings.Split(block, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
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
	plainTextHeadingPatterns = []struct {
		regex *regexp.Regexp
		level int
	}{
		{regexp.MustCompile(`^第[一二三四五六七八九十百千万0-9]+[章节部分篇].*$`), 1},
		{regexp.MustCompile(`^[一二三四五六七八九十百千万]+[、.．]\s*.+$`), 1},
		{regexp.MustCompile(`^\d+[、.．]\s*.+$`), 1},
		{regexp.MustCompile(`^[\(（][一二三四五六七八九十百千万0-9]+[\)）]\s*.+$`), 2},
		{regexp.MustCompile(`^\d+\.\d+(?:\.\d+)*\s*.+$`), 2},
		{regexp.MustCompile(`^(Q|q)\d+[:：]\s*.+$`), 2},
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
