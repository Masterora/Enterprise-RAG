package chatflow

import (
	"regexp"
	"strconv"

	"enterprise-rag/backend/internal/types"
)

var (
	knowledgeReferencePattern = regexp.MustCompile(`\[引用(\d+)\]`)
	externalReferencePattern  = regexp.MustCompile(`\[外链(\d+)\]`)
)

func ReferencedSources(answer string, chunks []types.RetrievalChunk, links []types.ExternalLink) ([]types.RetrievalChunk, []types.ExternalLink) {
	_, referencedChunks, referencedLinks := RemapReferencedSources(answer, chunks, links)
	return referencedChunks, referencedLinks
}

func RemapReferencedSources(answer string, chunks []types.RetrievalChunk, links []types.ExternalLink) (string, []types.RetrievalChunk, []types.ExternalLink) {
	if IsNoAnswer(answer) {
		return answer, nil, nil
	}
	chunkIndices := referenceIndices(knowledgeReferencePattern, answer)
	linkIndices := referenceIndices(externalReferencePattern, answer)
	answer = remapReferences(answer, knowledgeReferencePattern, chunkIndices)
	answer = remapReferences(answer, externalReferencePattern, linkIndices)
	return answer, selectChunks(chunkIndices, chunks), selectLinks(linkIndices, links)
}

func selectChunks(indices []int, chunks []types.RetrievalChunk) []types.RetrievalChunk {
	result := make([]types.RetrievalChunk, 0, len(indices))
	for _, index := range indices {
		if index >= 0 && index < len(chunks) {
			result = append(result, chunks[index])
		}
	}
	return result
}

func selectLinks(indices []int, links []types.ExternalLink) []types.ExternalLink {
	result := make([]types.ExternalLink, 0, len(indices))
	for _, index := range indices {
		if index >= 0 && index < len(links) {
			result = append(result, links[index])
		}
	}
	return result
}

func remapReferences(answer string, pattern *regexp.Regexp, indices []int) string {
	remapped := make(map[int]int, len(indices))
	for index, original := range indices {
		remapped[original] = index + 1
	}
	return pattern.ReplaceAllStringFunc(answer, func(reference string) string {
		match := pattern.FindStringSubmatch(reference)
		if len(match) != 2 {
			return reference
		}
		value, err := strconv.Atoi(match[1])
		if err != nil {
			return reference
		}
		mapped, ok := remapped[value-1]
		if !ok {
			return reference
		}
		if pattern == externalReferencePattern {
			return "[外链" + strconv.Itoa(mapped) + "]"
		}
		return "[引用" + strconv.Itoa(mapped) + "]"
	})
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
