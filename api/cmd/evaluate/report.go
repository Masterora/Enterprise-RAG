package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type summary struct {
	Total          int
	Succeeded      int
	Passed         int
	RecallCases    int
	RecallTotal    float64
	RouteCases     int
	RouteCorrect   int
	OutcomeCases   int
	OutcomeCorrect int
	CitationTotal  int
	Durations      []time.Duration
}

func summarize(results []caseResult) summary {
	result := summary{Total: len(results)}
	for _, item := range results {
		if item.Err != nil {
			continue
		}
		result.Succeeded++
		if item.Metrics.EvaluationPassed {
			result.Passed++
		}
		if item.ExpectedRecall {
			result.RecallCases++
			result.RecallTotal += item.Metrics.RecallAtK
		}
		if item.ExpectedRoute {
			result.RouteCases++
			if item.Metrics.RouteCorrect {
				result.RouteCorrect++
			}
		}
		if item.ExpectedOutcome {
			result.OutcomeCases++
			if item.Metrics.OutcomeCorrect {
				result.OutcomeCorrect++
			}
		}
		result.CitationTotal += item.Metrics.CitationCount
		result.Durations = append(result.Durations, item.Duration)
	}
	return result
}

func renderReport(s suite, results []caseResult, generatedAt time.Time) string {
	stats := summarize(results)
	var report strings.Builder
	report.WriteString("# RAG 质量评估报告\n\n")
	fmt.Fprintf(&report, "- 评估集：%s\n", displayName(s.Name))
	fmt.Fprintf(&report, "- 模式：%s\n", s.Mode)
	fmt.Fprintf(&report, "- 生成时间：%s\n", generatedAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- 用例：%d，API 成功：%d，评估通过：%d（%.1f%%）\n", stats.Total, stats.Succeeded, stats.Passed, percentage(stats.Passed, stats.Total))
	if stats.RecallCases > 0 {
		fmt.Fprintf(&report, "- 平均 Recall@K：%.1f%%（%d 个标注用例）\n", stats.RecallTotal/float64(stats.RecallCases)*100, stats.RecallCases)
	}
	if stats.RouteCases > 0 {
		fmt.Fprintf(&report, "- 路由正确率：%.1f%%（%d 个标注用例）\n", percentage(stats.RouteCorrect, stats.RouteCases), stats.RouteCases)
	}
	if stats.OutcomeCases > 0 {
		fmt.Fprintf(&report, "- 回答结果正确率：%.1f%%（%d 个标注用例）\n", percentage(stats.OutcomeCorrect, stats.OutcomeCases), stats.OutcomeCases)
	}
	if len(stats.Durations) > 0 {
		fmt.Fprintf(&report, "- 端到端延迟：P50 %s，P95 %s，P99 %s\n", percentile(stats.Durations, 0.50), percentile(stats.Durations, 0.95), percentile(stats.Durations, 0.99))
	}
	if s.Mode == modeAnswer && stats.Succeeded > 0 {
		fmt.Fprintf(&report, "- 平均引用数：%.2f\n", float64(stats.CitationTotal)/float64(stats.Succeeded))
	}

	report.WriteString("\n## 用例结果\n\n")
	report.WriteString("| 用例 | 状态 | Recall@K | 路由 | 回答结果 | 候选/返回 | 延迟 |\n")
	report.WriteString("| --- | --- | ---: | --- | --- | ---: | ---: |\n")
	for _, item := range results {
		if item.Err != nil {
			fmt.Fprintf(&report, "| %s | 失败：%s | - | - | - | - | %s |\n", escapeCell(item.Name), escapeCell(item.Err.Error()), formatDuration(item.Duration))
			continue
		}
		fmt.Fprintf(
			&report,
			"| %s | %s | %s | %s | %s | %d/%d | %s |\n",
			escapeCell(item.Name), passLabel(item.Metrics.EvaluationPassed), optionalPercent(item.ExpectedRecall, item.Metrics.RecallAtK),
			optionalBool(item.ExpectedRoute, item.Metrics.RouteCorrect), optionalBool(item.ExpectedOutcome, item.Metrics.OutcomeCorrect),
			item.Metrics.CandidateCount, item.Metrics.ReturnedCount, formatDuration(item.Duration),
		)
	}
	return report.String()
}

func percentile(values []time.Duration, quantile float64) string {
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	index := int(float64(len(ordered)-1)*quantile + 0.5)
	return formatDuration(ordered[index])
}

func percentage(value, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(value) / float64(total) * 100
}

func optionalPercent(enabled bool, value float64) string {
	if !enabled {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", value*100)
}

func optionalBool(enabled, value bool) string {
	if !enabled {
		return "-"
	}
	return passLabel(value)
}

func passLabel(value bool) string {
	if value {
		return "通过"
	}
	return "未通过"
}

func formatDuration(value time.Duration) string {
	if value < time.Second {
		return fmt.Sprintf("%dms", value.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", value.Seconds())
}

func displayName(value string) string {
	if value == "" {
		return "未命名评估集"
	}
	return value
}

func escapeCell(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "|", "\\|"), "\n", " ")
}
