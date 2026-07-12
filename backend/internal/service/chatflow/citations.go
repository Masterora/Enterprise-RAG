package chatflow

import (
	"regexp"
	"strconv"
	"strings"

	"enterprise-rag/backend/internal/types"
)

var (
	knowledgeReferencePattern = regexp.MustCompile(`\[引用(\d+)\]`)
	externalReferencePattern  = regexp.MustCompile(`\[外链(\d+)\]`)
)

func ReferencedSources(answer string, chunks []types.RetrievalChunk, links []types.ExternalLink) ([]types.RetrievalChunk, []types.ExternalLink) {
	if strings.TrimSpace(answer) == "无法确定" {
		return nil, nil
	}
	return referencedChunks(answer, chunks), referencedLinks(answer, links)
}

func referencedChunks(answer string, chunks []types.RetrievalChunk) []types.RetrievalChunk {
	indices := referenceIndices(knowledgeReferencePattern, answer)
	result := make([]types.RetrievalChunk, 0, len(indices))
	for _, index := range indices {
		if index >= 0 && index < len(chunks) {
			result = append(result, chunks[index])
		}
	}
	return result
}

func referencedLinks(answer string, links []types.ExternalLink) []types.ExternalLink {
	indices := referenceIndices(externalReferencePattern, answer)
	result := make([]types.ExternalLink, 0, len(indices))
	for _, index := range indices {
		if index >= 0 && index < len(links) {
			result = append(result, links[index])
		}
	}
	return result
}

func referenceIndices(pattern *regexp.Regexp, answer string) []int {
	seen := make(map[int]struct{})
	result := make([]int, 0)
	for _, match := range pattern.FindAllStringSubmatch(answer, -1) {
		value, err := strconv.Atoi(match[1])
		if err != nil || value < 1 {
			continue
		}
		index := value - 1
		if _, ok := seen[index]; ok {
			continue
		}
		seen[index] = struct{}{}
		result = append(result, index)
	}
	return result
}
