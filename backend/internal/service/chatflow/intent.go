package chatflow

import (
	"strings"
	"unicode"
)

func decideRoute(query string) routeDecision {
	normalized := strings.TrimSpace(strings.ToLower(query))
	if normalized == "" {
		return routeDecision{route: QueryRouteRAG}
	}

	overviewScore := scoreOverviewIntent(normalized)
	navigationScore := scoreNavigationIntent(normalized)
	best := routeDecision{route: QueryRouteRAG}
	if overviewScore >= navigationScore {
		best.route = QueryRouteOverview
		best.score = overviewScore
		best.competingScore = navigationScore
	} else {
		best.route = QueryRouteNavigation
		best.score = navigationScore
		best.competingScore = overviewScore
	}
	if best.score < 3 || best.score-best.competingScore < 2 {
		return routeDecision{route: QueryRouteRAG, score: best.score, competingScore: best.competingScore}
	}
	return best
}

func scoreOverviewIntent(query string) int {
	score := 0
	if containsAny(query,
		"这个知识库有什么内容", "知识库有什么内容", "库里有什么内容", "这个库里有什么",
		"知识库概览", "内容概览", "整体内容", "整体讲什么", "整体介绍",
	) {
		score += 6
	}
	if containsAny(query,
		"主要讲什么", "主要包含什么", "包含哪些内容", "都是什么内容", "覆盖哪些方向", "有什么内容",
		"解决什么问题", "能解决什么问题", "解决哪些问题", "能做什么", "用来做什么", "适合做什么", "有什么用途", "应用场景",
	) {
		score += 4
	}
	if containsAny(query, "知识库", "这个库", "库里") {
		score += 2
	}
	if containsAny(query, "为什么", "怎么", "如何", "是什么", "作用", "区别", "优势", "流程", "原理") {
		score -= 3
	}
	if containsAny(query, "哪些文档", "哪篇文档", "哪份文档", "相关文档", "哪些文件", "哪些资料") {
		score -= 4
	}
	return score
}

func scoreNavigationIntent(query string) int {
	score := 0
	if containsAny(query,
		"有哪些文档", "哪些文档", "什么文档", "哪些资料", "哪些文件",
		"哪篇文档", "哪份文档", "相关文档", "相关资料", "哪个文件", "哪份资料",
	) {
		score += 6
	}
	if containsAny(query, "在哪个文档", "在哪篇文档", "可优先看", "看哪篇", "看哪个文件", "来源于哪篇文档") {
		score += 5
	}
	if containsAny(query, "文档", "资料", "文件") {
		score += 2
	}
	if containsAny(query, "这个知识库", "知识库概览", "整体内容") {
		score -= 4
	}
	if containsAny(query, "为什么", "怎么", "如何", "是什么", "作用", "区别", "优势") &&
		!containsAny(query, "在哪个文档", "哪篇文档", "哪些文档", "相关文档") {
		score -= 3
	}
	return score
}

func routeKeywordTokens(text string) []string {
	text = routeNormalizeText(text)
	tokens := make([]string, 0)
	var builder strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			continue
		}
		if builder.Len() > 0 {
			tokens = appendRouteToken(tokens, builder.String())
			builder.Reset()
		}
	}
	if builder.Len() > 0 {
		tokens = appendRouteToken(tokens, builder.String())
	}
	return tokens
}

func appendRouteToken(tokens []string, token string) []string {
	runes := []rune(token)
	if len(runes) < 2 {
		return tokens
	}
	if routeContainsHan(runes) {
		for i := 0; i+1 < len(runes); i++ {
			tokens = append(tokens, string(runes[i:i+2]))
		}
		tokens = append(tokens, token)
		return tokens
	}
	return append(tokens, token)
}

func routeContainsHan(runes []rune) bool {
	for _, r := range runes {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func routeNormalizeText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer("？", "", "?", "", "。", "", ".", "", "，", "", ",", "", "：", "", ":", "", "（", "", "）", "", "(", "", ")", "")
	return replacer.Replace(text)
}
