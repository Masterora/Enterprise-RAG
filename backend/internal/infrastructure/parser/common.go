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
	if len(*segments) > 0 && (*segments)[len(*segments)-1].Section == section {
		(*segments)[len(*segments)-1].Content += "\n\n" + content
	} else {
		*segments = append(*segments, model.ParseSegment{
			Page:    0,
			Section: section,
			Content: content,
		})
	}
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
