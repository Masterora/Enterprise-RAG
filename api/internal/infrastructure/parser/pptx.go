package parser

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/task"
)

func parsePPTX(data []byte, options ParseOptions) (string, []byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, err
	}

	slideNames := listSlideXMLNames(reader)
	if len(slideNames) == 0 {
		return "", nil, errors.New("pptx slides not found")
	}

	var (
		plainText strings.Builder
		segments  []model.ParseSegment
	)

	for index, slideName := range slideNames {
		slideXML, err := readZipFileByAlias(reader, slideName, "pptx slide")
		if err != nil {
			return "", nil, err
		}

		paragraphs, err := extractPPTXParagraphs(slideXML)
		if err != nil {
			return "", nil, err
		}
		if len(paragraphs) == 0 {
			continue
		}

		title, content := buildPPTXSlideContent(paragraphs)
		section := fmt.Sprintf("第%d页", index+1)
		if title != "" {
			section += " / " + title
		}
		appendParseSegment(&plainText, &segments, section, content)
	}

	if options.ProcessingMode == task.ProcessingModeEnhanced && len(segments) == 0 && zipContainsPrefix(reader, "ppt/media/") {
		return "", nil, errors.New("演示文稿未提取到可读文字，疑似以图片内容为主，当前未启用 OCR")
	}
	if len(segments) == 0 {
		return "", nil, errors.New("presentation has no readable content")
	}
	return finalizeParseResult(plainText.String(), segments)
}

func listSlideXMLNames(reader *zip.Reader) []string {
	slideNames := make([]string, 0)
	for _, file := range reader.File {
		if !pptxSlidePathPattern.MatchString(file.Name) {
			continue
		}
		slideNames = append(slideNames, file.Name)
	}
	sort.SliceStable(slideNames, func(i, j int) bool {
		return slideIndexOf(slideNames[i]) < slideIndexOf(slideNames[j])
	})
	return slideNames
}

func slideIndexOf(name string) int {
	match := pptxSlidePathPattern.FindStringSubmatch(name)
	if len(match) != 2 {
		return 0
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return value
}

func extractPPTXParagraphs(data []byte) ([]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	paragraphs := make([]string, 0)
	var (
		inTable       bool
		inParagraph   bool
		currentRow    []string
		tableRows     []string
		cellParts     []string
		paragraphText strings.Builder
	)

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "tbl":
				inTable = true
				tableRows = tableRows[:0]
			case "tr":
				if inTable {
					currentRow = currentRow[:0]
				}
			case "tc":
				if inTable {
					cellParts = cellParts[:0]
				}
			case "p":
				if value.Name.Space == drawingMLNamespace {
					inParagraph = true
					paragraphText.Reset()
				}
			case "t":
				var text string
				if err := decoder.DecodeElement(&text, &value); err != nil {
					return nil, err
				}
				text = normalizeTextBlock(text)
				if text == "" {
					continue
				}
				if inTable {
					cellParts = append(cellParts, text)
					continue
				}
				if inParagraph {
					if paragraphText.Len() > 0 {
						paragraphText.WriteString(" ")
					}
					paragraphText.WriteString(text)
				}
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "p":
				if value.Name.Space != drawingMLNamespace || !inParagraph || inTable {
					continue
				}
				inParagraph = false
				text := normalizeTextBlock(paragraphText.String())
				if text != "" {
					paragraphs = append(paragraphs, text)
				}
			case "tc":
				if inTable && len(cellParts) > 0 {
					currentRow = append(currentRow, strings.Join(cellParts, " "))
				}
			case "tr":
				if inTable && len(currentRow) > 0 {
					tableRows = append(tableRows, strings.Join(currentRow, " | "))
				}
			case "tbl":
				if inTable && len(tableRows) > 0 {
					paragraphs = append(paragraphs, strings.Join(tableRows, "\n"))
				}
				inTable = false
			}
		}
	}
	return paragraphs, nil
}

func buildPPTXSlideContent(paragraphs []string) (string, string) {
	title := ""
	bodyStart := 0
	if len(paragraphs) > 0 && looksLikePPTXTitle(paragraphs[0]) {
		title = cleanHeadingText(paragraphs[0])
		bodyStart = 1
	}

	bodyParts := make([]string, 0, len(paragraphs)-bodyStart)
	for _, paragraph := range paragraphs[bodyStart:] {
		paragraph = normalizeTextBlock(paragraph)
		if paragraph == "" {
			continue
		}
		bodyParts = append(bodyParts, paragraph)
	}

	if len(bodyParts) == 0 && title != "" {
		bodyParts = append(bodyParts, title)
		title = ""
	}

	return title, strings.Join(bodyParts, "\n\n")
}

func looksLikePPTXTitle(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if len([]rune(text)) > 40 {
		return false
	}
	if strings.Contains(text, " | ") || strings.Contains(text, "\n") {
		return false
	}
	return !plainTextListMarkerPattern.MatchString(text)
}

var pptxSlidePathPattern = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)

const drawingMLNamespace = "http://schemas.openxmlformats.org/drawingml/2006/main"
