package parser

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"enterprise-rag/backend/internal/model"
)

func finalizeParseResult(plainText string, segments []model.ParseSegment) (string, []byte, error) {
	plainText = strings.TrimSpace(plainText)
	metadata, err := json.Marshal(model.DocumentMetadata{Segments: segments})
	if err != nil {
		return "", nil, err
	}
	return plainText, metadata, nil
}

func appendParseSegment(plainText *strings.Builder, segments *[]model.ParseSegment, section, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	if strings.TrimSpace(section) == "" {
		section = "text"
	}
	headingPath := splitHeadingPath(section)
	blockType := detectBlockType(content)
	if len(*segments) > 0 && (*segments)[len(*segments)-1].Section == section {
		(*segments)[len(*segments)-1].Content += "\n\n" + content
		if (*segments)[len(*segments)-1].BlockType != blockType {
			(*segments)[len(*segments)-1].BlockType = "mixed"
		}
	} else {
		*segments = append(*segments, model.ParseSegment{
			Page:        0,
			Section:     section,
			HeadingPath: headingPath,
			BlockType:   blockType,
			Content:     content,
		})
	}
	if plainText.Len() > 0 {
		plainText.WriteString("\n\n")
	}
	plainText.WriteString(content)
}

func splitHeadingPath(section string) []string {
	if strings.TrimSpace(section) == "" || strings.EqualFold(strings.TrimSpace(section), "text") {
		return nil
	}
	parts := strings.Split(section, " / ")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func detectBlockType(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
		return "code"
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) >= 2 && strings.Contains(lines[0], "|") && strings.Contains(lines[1], "|") {
		return "table"
	}
	if len(lines) > 1 {
		listLines := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || plainTextListMarkerPattern.MatchString(line) {
				listLines++
			}
		}
		if listLines*2 >= len(lines) {
			return "list"
		}
	}
	return "paragraph"
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

func readZipFileByAlias(reader *zip.Reader, name, alias string) ([]byte, error) {
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer handle.Close()
		return io.ReadAll(handle)
	}
	return nil, errors.New(alias + " not found")
}

func zipContainsPrefix(reader *zip.Reader, prefix string) bool {
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, prefix) {
			return true
		}
	}
	return false
}
