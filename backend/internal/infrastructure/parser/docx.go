package parser

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"

	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/task"
)

func parseDOCX(data []byte, options ParseOptions) (string, []byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, err
	}

	documentXML, err := readZipFile(reader, "word/document.xml")
	if err != nil {
		return "", nil, err
	}

	body, err := extractDOCXBody(documentXML)
	if err != nil {
		return "", nil, err
	}

	var (
		plainText strings.Builder
		segments  []model.ParseSegment
		headings  []string
	)

	for _, block := range body {
		switch block.kind {
		case docxBlockHeading:
			headings = trimHeadingPath(headings, block.level)
			headings = append(headings, cleanHeadingText(block.content))
		case docxBlockParagraph, docxBlockTable:
			appendParseSegment(&plainText, &segments, currentSection(headings), block.content)
		}
	}

	if options.ProcessingMode == task.ProcessingModeEnhanced && len(segments) == 0 && zipContainsPrefix(reader, "word/media/") {
		return "", nil, errors.New("文档未提取到可读文字，疑似以图片内容为主，当前未启用 OCR")
	}
	if len(segments) == 0 {
		return "", nil, errors.New("document has no readable content")
	}
	return finalizeParseResult(plainText.String(), segments)
}

func readZipFile(reader *zip.Reader, name string) ([]byte, error) {
	return readZipFileByAlias(reader, name, "docx document.xml")
}

type docxBlockKind int

const (
	docxBlockParagraph docxBlockKind = iota + 1
	docxBlockHeading
	docxBlockTable
)

type docxBlock struct {
	kind    docxBlockKind
	level   int
	content string
}

func extractDOCXBody(data []byte) ([]docxBlock, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	blocks := make([]docxBlock, 0)

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return blocks, nil
		}
		if err != nil {
			return nil, err
		}

		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		switch start.Name.Local {
		case "p":
			block, err := parseDOCXParagraph(decoder, start)
			if err != nil {
				return nil, err
			}
			if block != nil {
				blocks = append(blocks, *block)
			}
		case "tbl":
			block, err := parseDOCXTable(decoder, start)
			if err != nil {
				return nil, err
			}
			if block != nil {
				blocks = append(blocks, *block)
			}
		}
	}
}

func parseDOCXParagraph(decoder *xml.Decoder, start xml.StartElement) (*docxBlock, error) {
	var (
		textBuilder strings.Builder
		styleName   string
		isList      bool
	)

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "pStyle":
				styleName = attrValue(value.Attr, "val")
			case "numPr":
				isList = true
			case "t":
				var text string
				if err := decoder.DecodeElement(&text, &value); err != nil {
					return nil, err
				}
				textBuilder.WriteString(text)
			case "tab":
				textBuilder.WriteString("\t")
			case "br", "cr":
				textBuilder.WriteString("\n")
			}
		case xml.EndElement:
			if value.Name.Local != start.Name.Local {
				continue
			}

			content := normalizeTextBlock(textBuilder.String())
			if content == "" {
				return nil, nil
			}
			if level, ok := docxHeadingLevel(styleName, content, isList); ok {
				return &docxBlock{kind: docxBlockHeading, level: level, content: content}, nil
			}
			if isList && !plainTextListMarkerPattern.MatchString(content) {
				content = "- " + content
			}
			return &docxBlock{kind: docxBlockParagraph, content: content}, nil
		}
	}
}

func parseDOCXTable(decoder *xml.Decoder, start xml.StartElement) (*docxBlock, error) {
	var rows []string

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local != "tr" {
				continue
			}
			row, err := parseDOCXTableRow(decoder, value)
			if err != nil {
				return nil, err
			}
			if row != "" {
				rows = append(rows, row)
			}
		case xml.EndElement:
			if value.Name.Local != start.Name.Local {
				continue
			}
			content := strings.TrimSpace(strings.Join(rows, "\n"))
			if content == "" {
				return nil, nil
			}
			return &docxBlock{kind: docxBlockTable, content: content}, nil
		}
	}
}

func parseDOCXTableRow(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	cells := make([]string, 0)

	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}

		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local != "tc" {
				continue
			}
			cell, err := parseDOCXTableCell(decoder, value)
			if err != nil {
				return "", err
			}
			if cell != "" {
				cells = append(cells, cell)
			}
		case xml.EndElement:
			if value.Name.Local != start.Name.Local {
				continue
			}
			return strings.Join(cells, " | "), nil
		}
	}
}

func parseDOCXTableCell(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	values := make([]string, 0)

	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}

		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local != "p" {
				continue
			}
			block, err := parseDOCXParagraph(decoder, value)
			if err != nil {
				return "", err
			}
			if block != nil && block.content != "" {
				values = append(values, block.content)
			}
		case xml.EndElement:
			if value.Name.Local != start.Name.Local {
				continue
			}
			return strings.Join(values, "\n"), nil
		}
	}
}

func docxHeadingLevel(styleName, content string, isList bool) (int, bool) {
	styleName = strings.TrimSpace(strings.ToLower(styleName))
	if strings.HasPrefix(styleName, "heading") {
		levelText := strings.TrimPrefix(styleName, "heading")
		if level, err := strconv.Atoi(levelText); err == nil && level > 0 {
			return level, true
		}
		return 1, true
	}
	if isList {
		return 0, false
	}
	return detectPlainTextHeading(content)
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, attr := range attrs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}
