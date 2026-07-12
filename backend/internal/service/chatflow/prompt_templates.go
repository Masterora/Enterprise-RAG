package chatflow

import (
	"fmt"
	"strings"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/types"
)

const defaultAnswerTemplate = `你是企业知识库问答助手。
回答规则：
1. 只能使用资料片段中的事实，不要编造，不要补充常识背景。
2. 如果资料不足以回答，必须只回答：无法确定。
3. 回答必须正式、完整、信息充分，先给核心结论，再给必要说明。
4. 优先引用最相关的资料片段，忽略仅是目录、测试题、清单、术语表之类的弱相关内容。
5. 如果使用分点，控制在 3 到 5 点；不要出现空行分隔的松散列表。
6. 回答必须使用中文；除非用户明确要求，否则不要输出英文术语解释。
7. 所有回答内容要自然融为一体，不要写“基于知识库”“外部资料补充说明”“同类方案参考”这类分段标题。
8. 引用标注只放在句末或段末，不要单独成行，不要每一句都标注。
9. 每个自然段最多标 1 到 2 个引用，优先保留最关键的依据。
10. 严格紧凑排版：不要使用标题样式、不要在段落之间插入空白行、不要输出孤立的短句行。
11. 如果使用编号列表，必须写成“1. 内容”这种单行编号，不要把“1.”单独放一行。
12. 不要输出“核心结论：”“补充说明如下：”这类空洞标题，直接进入正文。
13. 不要输出资料片段之外的来源。

问题：
{{question}}

知识库资料片段：
{{knowledge_chunks}}

请直接给出最终答案。若可以确定，请优先在关键结论后使用 [引用1] 这类格式标注依据。`

const defaultExplanationTemplate = `你是企业知识库问答助手。
回答规则：
1. 只能使用资料片段中的事实，不得用常识补齐缺失原因。
2. 先直接说明原因、机制或差异，再按必要的因果顺序展开；不要只摘录操作步骤。
3. 如果多个相邻片段共同构成原因，应综合说明，但不得超出片段能支持的结论。
4. 资料不足时必须只回答：无法确定。
5. 回答使用中文，正式、紧凑，不要输出空洞标题或段落间空白行。
6. 引用只放在能够直接支持结论的句末，每段最多 1 到 2 个引用。
7. 不要输出资料片段之外的来源。

问题：
{{question}}

知识库资料片段：
{{knowledge_chunks}}

请直接给出完整答案，并使用 [引用1] 这类格式标注关键依据。`

const defaultWebSearchTemplate = `你是企业知识库问答助手。
回答规则：
1. 先基于知识库资料回答，再结合外部网页资料补充必要信息。
2. 只要提供了外部网页资料，就必须把其中有价值的信息纳入回答；不要忽略。
3. 联网信息必须作为补充和参考依据，不能冒充知识库资料；资料和网络都不足时，只回答：无法确定。
4. 回答必须正式、完整、信息充分，先给核心结论，再给必要说明。
5. 优先引用最相关的资料片段，忽略仅是目录、测试题、清单、术语表之类的弱相关内容。
6. 如果使用分点，控制在 3 到 5 点；不要出现空行分隔的松散列表。
7. 回答必须使用中文；除非用户明确要求，否则不要输出英文术语解释。
8. 所有回答内容要自然融为一体，不要写“基于知识库”“外部资料补充说明”“同类方案参考”这类分段标题。
9. 引用标注只放在句末或段末，不要单独成行，不要每一句都标注。
10. 每个自然段最多标 1 到 2 个引用，优先保留最关键的依据。
11. 严格紧凑排版：不要使用标题样式、不要在段落之间插入空白行、不要输出孤立的短句行。
12. 如果使用编号列表，必须写成“1. 内容”这种单行编号，不要把“1.”单独放一行。
13. 不要输出“核心结论：”“补充说明如下：”这类空洞标题，直接进入正文。
14. 如果引用知识库资料，请用 [引用1] 这类格式；如果引用外部网页，请用 [外链1] 这类格式。
15. 不要把网页内容表述为知识库资料。

问题：
{{question}}

知识库资料片段：
{{knowledge_chunks}}

外部网页资料：
{{external_links}}

请直接给出最终答案。若可以确定，请优先在关键结论后使用 [引用1] 或 [外链1] 这类格式标注依据。`

const defaultOverviewPolishTemplate = `你是企业知识库概览润色助手。
请围绕用户问题，在不改变事实与引用编号的前提下，把下面的知识库概览草稿整理成更自然、更紧凑、与问题更贴合的最终回答。
要求：
1. 必须围绕用户问题作答；如果问题偏“用途/能解决什么问题/适合做什么”，就优先总结应用场景、适用工作和能支撑的问题类型。
2. 只允许依据草稿和核对资料，不要补充外部常识，不要编造。
3. 可以调整表达和句子顺序，但不要删除引用编号，也不要让引用单独成行。
4. 不要输出空洞标题，不要拆成多余小节。
5. 回答必须中文、紧凑、正式。

用户问题：
{{question}}

概览草稿：
{{draft}}

核对资料：
{{supporting_chunks}}

请直接输出润色后的最终答案，不要解释你的修改。`

const defaultRouteTemplate = `你是企业知识库问答系统的意图路由器。
请根据用户在已选择知识库中的提问，判断唯一处理方式：
- rag：询问知识库中的具体事实、原因、流程、定义、比较或技术细节。
- overview：询问整个知识库有什么内容、能做什么、能解决什么问题或适用场景。短问中的“它”“这个”“能干什么”等省略表达默认指当前知识库。
- navigation：查找哪些文档、文件或资料涉及某个主题，或询问应该阅读哪份资料。
- fallback：纯寒暄、与知识库无关的闲聊，或没有实际信息需求的输入。

如果 route 是 rag，把问题改写为适合知识库检索的一行查询，保留原意并补充同义表达和关键实体；如果 route 是 navigation，只提取要查找的主题或实体；其他 route 的 search_query 返回空字符串。不得回答问题或编造事实，长度不超过 80 字。
只输出 JSON，不要解释，不要 Markdown：{"route":"rag","search_query":"检索查询"}

用户问题：
{{question}}`

func BuildPrompt(promptConf config.PromptConf, query string, chunks []types.RetrievalChunk, externalLinks []types.ExternalLink, webSearch bool) string {
	template := strings.TrimSpace(promptConf.AnswerTemplate)
	if webSearch {
		template = strings.TrimSpace(promptConf.WebSearchTemplate)
	} else if isExplanationQuery(query) {
		template = strings.TrimSpace(promptConf.ExplanationTemplate)
	}
	if template == "" {
		if webSearch {
			template = defaultWebSearchTemplate
		} else if isExplanationQuery(query) {
			template = defaultExplanationTemplate
		} else {
			template = defaultAnswerTemplate
		}
	}

	return renderPromptTemplate(template, map[string]string{
		"question":         query,
		"knowledge_chunks": formatKnowledgeChunks(chunks),
		"external_links":   formatExternalLinks(externalLinks),
	})
}

func isExplanationQuery(query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	return containsAny(query, "为什么", "为何", "原因", "怎么", "如何", "流程", "原理", "机制", "区别", "差异", "优势", "影响", "风险")
}

func BuildOverviewPolishPrompt(promptConf config.PromptConf, query, draft string, summaries []overviewDocSummary) string {
	template := strings.TrimSpace(promptConf.OverviewPolishTemplate)
	if template == "" {
		template = defaultOverviewPolishTemplate
	}

	return renderPromptTemplate(template, map[string]string{
		"question":          query,
		"draft":             draft,
		"supporting_chunks": formatOverviewSupportingChunks(summaries),
	})
}

func BuildRoutePrompt(promptConf config.PromptConf, query string) string {
	template := strings.TrimSpace(promptConf.RouteTemplate)
	if template == "" {
		template = defaultRouteTemplate
	}
	return renderPromptTemplate(template, map[string]string{"question": query})
}

func renderPromptTemplate(template string, values map[string]string) string {
	replacements := make([]string, 0, len(values)*2)
	for key, value := range values {
		replacements = append(replacements, "{{"+key+"}}", strings.TrimSpace(value))
	}
	return strings.NewReplacer(replacements...).Replace(template)
}

func formatKnowledgeChunks(chunks []types.RetrievalChunk) string {
	if len(chunks) == 0 {
		return "无。"
	}

	var builder strings.Builder
	for index, chunk := range chunks {
		docName := strings.TrimSpace(chunk.DocName)
		if docName == "" {
			docName = "未知"
		}
		section := strings.TrimSpace(chunk.Section)
		if section == "" {
			section = "未知"
		}
		page := "无"
		if chunk.Page > 0 {
			page = fmt.Sprintf("%d", chunk.Page)
		}
		builder.WriteString(fmt.Sprintf("[引用%d]\n文档：%s\n页码：%s\n章节：%s\n内容：%s\n\n",
			index+1, docName, page, section, chunk.Content))
	}
	return strings.TrimSpace(builder.String())
}

func formatExternalLinks(externalLinks []types.ExternalLink) string {
	if len(externalLinks) == 0 {
		return "无。\n注意：当前没有可用外部网页资料，不要输出“外部网页资料补充说明”或编造外部来源。"
	}

	var builder strings.Builder
	for index, link := range externalLinks {
		title := strings.TrimSpace(link.Title)
		if title == "" {
			title = "未知"
		}
		url := strings.TrimSpace(link.URL)
		if url == "" {
			url = "未知"
		}
		snippet := strings.TrimSpace(link.Snippet)
		if snippet == "" {
			snippet = "未知"
		}
		builder.WriteString(fmt.Sprintf("[外链%d]\n标题：%s\n链接：%s\n摘要：%s\n\n",
			index+1, title, url, snippet))
	}
	return strings.TrimSpace(builder.String())
}

func formatOverviewSupportingChunks(summaries []overviewDocSummary) string {
	var builder strings.Builder
	for index, summary := range summaries {
		sectionText := cleanOverviewSections(summary.sections)
		if sectionText == "" {
			sectionText = "相关协议与说明"
		}
		content := compactRunes(strings.TrimSpace(summary.chunk.Content), 180)
		builder.WriteString(fmt.Sprintf("[引用%d]\n文档：%s\n主题线索：%s\n片段：%s\n\n",
			index+1, summary.document.Filename, sectionText, content))
	}
	return strings.TrimSpace(builder.String())
}
