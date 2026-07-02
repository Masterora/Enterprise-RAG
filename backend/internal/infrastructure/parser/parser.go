package parser

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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
