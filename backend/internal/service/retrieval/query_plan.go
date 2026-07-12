package retrieval

import (
	"strings"
	"unicode"
)

type queryPlan struct {
	original string
	queries  []string
}

func buildQueryPlan(original, rewritten string, maxRunes, maxSubQueries int) queryPlan {
	if maxRunes < 1 {
		maxRunes = 500
	}
	original = strings.Join(strings.Fields(original), " ")
	rewritten = compactQuery(rewritten, maxRunes)
	if maxSubQueries < 1 {
		maxSubQueries = 4
	}

	queries := make([]string, 0, maxSubQueries)
	queries = appendUniqueQuery(queries, rewritten)
	parts := splitQuestion(original)
	if len([]rune(original)) > maxRunes && len(parts) == 1 {
		parts = splitLongQuestion(original)
	}
	for _, part := range parts {
		queries = appendUniqueQuery(queries, compactQuery(part, maxRunes))
		if len(queries) >= maxSubQueries {
			break
		}
	}
	if len(queries) == 0 {
		queries = append(queries, original)
	}
	return queryPlan{original: original, queries: queries}
}

func splitLongQuestion(query string) []string {
	parts := strings.FieldsFunc(query, func(r rune) bool {
		switch r {
		case '。', '！', '!', '?', '？', ';', '；', '\n':
			return true
		}
		return false
	})
	if len(parts) == 0 {
		return []string{query}
	}
	return parts
}

func splitQuestion(query string) []string {
	parts := strings.FieldsFunc(query, func(r rune) bool {
		switch r {
		case '?', '？', ';', '；', '\n':
			return true
		}
		return false
	})
	if len(parts) == 1 {
		for _, separator := range []string{"另外", "同时", "以及", "并且", "还想知道"} {
			if strings.Contains(query, separator) {
				parts = strings.Split(query, separator)
				break
			}
		}
	}
	return parts
}

func compactQuery(query string, maxRunes int) string {
	query = strings.Join(strings.Fields(query), " ")
	if maxRunes < 1 {
		maxRunes = 500
	}
	runes := []rune(query)
	if len(runes) <= maxRunes {
		return query
	}
	cut := maxRunes
	for index := maxRunes; index > maxRunes*3/4; index-- {
		if unicode.IsPunct(runes[index-1]) || unicode.IsSpace(runes[index-1]) {
			cut = index
			break
		}
	}
	return strings.TrimSpace(string(runes[:cut]))
}

func appendUniqueQuery(queries []string, query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return queries
	}
	for _, existing := range queries {
		if strings.EqualFold(existing, query) {
			return queries
		}
	}
	return append(queries, query)
}
