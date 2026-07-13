import { streamChat, type AgentStep } from '../../api/chat'
import { truncateText } from './utils'

export type ChatFailure = {
  status: string
  detail: string
  toast: string
}

type StreamCallbacks = {
  updateMessage: (sessionID: string, messageID: string, patch: Record<string, unknown>) => void
  appendProcessStep: (sessionID: string, messageID: string, step: string) => void
  appendAnswer: (sessionID: string, messageID: string, content: string) => void
  updateAgentStep: (sessionID: string, messageID: string, step: AgentStep) => void
}

type StreamLabels = {
  rewriteDone: (query: string) => string
  rewriteSkipped: string
  retrievalDone: (returned: number, candidates: number) => string
  rerankDone: string
  rerankSkipped: string
  sourcesDone: (count: number) => string
  webSourcesDone: (count: number) => string
  answerStreaming: string
  finished: string
  done: string
}

type StreamInput = {
  sessionID: string
  messageID: string
  question: string
  subjectID: string
  llmProvider: string
  llmModel: string
  webSearch: boolean
  topK: number
  askTimeoutMS: number
  labels: StreamLabels
  callbacks: StreamCallbacks
  localizeStatus: (status: string) => string
}

export async function streamAnswer(input: StreamInput) {
  const controller = new AbortController()
  let completed = false
  let timeout = window.setTimeout(() => controller.abort(), input.askTimeoutMS)
  const resetTimeout = () => {
    window.clearTimeout(timeout)
    timeout = window.setTimeout(() => controller.abort(), input.askTimeoutMS)
  }
  try {
    await streamChat(
      {
          session_id: input.sessionID,
          message_id: input.messageID,
          subject_id: input.subjectID,
          query: input.question,
          top_k: input.topK,
          llm_provider: input.llmProvider,
          llm_model: input.llmModel,
          web_search: input.webSearch,
      },
      {
          onEvent: resetTimeout,
          onStatus: (status) => {
            const localizedStatus = input.localizeStatus(status)
            input.callbacks.updateMessage(input.sessionID, input.messageID, { status: localizedStatus })
            input.callbacks.appendProcessStep(input.sessionID, input.messageID, localizedStatus)
          },
          onAgentStep: (step) => {
            input.callbacks.updateAgentStep(input.sessionID, input.messageID, step)
          },
          onSources: (chunks) => {
            input.callbacks.updateMessage(input.sessionID, input.messageID, { chunks })
            input.callbacks.appendProcessStep(input.sessionID, input.messageID, input.labels.sourcesDone(chunks.length))
          },
          onWebSources: (links) => {
            input.callbacks.updateMessage(input.sessionID, input.messageID, { externalLinks: links })
            input.callbacks.appendProcessStep(input.sessionID, input.messageID, input.labels.webSourcesDone(links.length))
          },
          onMetrics: (metrics) => {
            input.callbacks.updateMessage(input.sessionID, input.messageID, { metrics })
            input.callbacks.appendProcessStep(
              input.sessionID,
              input.messageID,
              metrics.query_rewritten
                ? input.labels.rewriteDone(truncateText(metrics.search_query, 80))
                : input.labels.rewriteSkipped,
            )
            input.callbacks.appendProcessStep(
              input.sessionID,
              input.messageID,
              input.labels.retrievalDone(metrics.returned_count, metrics.candidate_count),
            )
            input.callbacks.appendProcessStep(
              input.sessionID,
              input.messageID,
              metrics.reranked ? input.labels.rerankDone : input.labels.rerankSkipped,
            )
          },
          onDelta: (content) => {
            input.callbacks.appendProcessStep(input.sessionID, input.messageID, input.labels.answerStreaming)
            input.callbacks.appendAnswer(input.sessionID, input.messageID, content)
          },
          onDone: (answer) => {
            completed = true
            input.callbacks.appendProcessStep(input.sessionID, input.messageID, input.labels.finished)
            input.callbacks.updateMessage(input.sessionID, input.messageID, {
              status: input.labels.done,
              ...(answer ? { answer } : {}),
              errorReason: '',
              finishedAt: Date.now(),
              loading: false,
            })
          },
      },
      controller.signal,
    )
  } catch (error) {
    if (completed) {
      return
    }
    throw error
  } finally {
    window.clearTimeout(timeout)
  }
}

const chatStatusKeys: Record<string, string> = {
  'agent.plan.start': 'chat.status.agentPlan',
  'agent.answer.start': 'chat.status.agentAnswer',
  'chat.route.overview': 'chat.status.routeOverview',
  'chat.route.navigation': 'chat.status.routeNavigation',
  'chat.route.fallback': 'chat.status.routeFallback',
  'chat.retrieval.start': 'chat.status.retrievalStart',
  'chat.answer.insufficient': 'chat.status.answerInsufficient',
  'chat.answer.generating': 'chat.status.answerGenerating',
  'chat.web.prepare': 'chat.status.webPrepare',
  'chat.web.searching': 'chat.status.webSearching',
  'chat.web.ready': 'chat.status.webReady',
  'chat.web.empty': 'chat.status.webEmpty',
  'retrieval.rewrite.start': 'chat.status.rewriteStart',
  'retrieval.rewrite.fallback': 'chat.status.rewriteFallback',
  'retrieval.rewrite.done': 'chat.status.rewriteDone',
  'retrieval.rewrite.skipped': 'chat.status.rewriteSkipped',
  'retrieval.rewrite.disabled': 'chat.status.rewriteDisabled',
  'retrieval.query.split': 'chat.status.querySplit',
  'retrieval.embedding.start': 'chat.status.embeddingStart',
  'retrieval.vector.start': 'chat.status.vectorStart',
  'retrieval.keyword.start': 'chat.status.keywordStart',
  'retrieval.merge.start': 'chat.status.mergeStart',
  'retrieval.rerank.start': 'chat.status.rerankStart',
  'retrieval.rerank.skipped': 'chat.status.rerankSkipped',
  'retrieval.citations.trim': 'chat.status.citationsTrim',
}

export function localizeChatStatus(status: string, t: (key: string) => string) {
  const key = chatStatusKeys[status]
  return key ? t(key) : t('chat.status.processing')
}

export function classifyChatFailure(error: unknown, t: (key: string) => string): ChatFailure {
  const rawMessage =
    error instanceof Error ? error.message : typeof error === 'string' ? error : t('chat.failedToast')
  const normalized = rawMessage.toLowerCase()

  if (
    normalized.includes('timeout') ||
    normalized.includes('aborted') ||
    normalized.includes('deadline exceeded') ||
    normalized.includes('context deadline exceeded')
  ) {
    return {
      status: t('chat.failure.timeoutStatus'),
      detail: t('chat.failure.timeoutDetail'),
      toast: t('chat.failure.timeoutToast'),
    }
  }
  if (
    normalized.includes('invalid api key') ||
    normalized.includes('incorrect api key') ||
    normalized.includes('invalid_api_key') ||
    normalized.includes('insufficient credits') ||
    normalized.includes('402')
  ) {
    return {
      status: t('chat.failure.configStatus'),
      detail: t('chat.failure.configDetail'),
      toast: t('chat.failure.configToast'),
    }
  }
  if (normalized.includes('no rows in result set')) {
    return {
      status: t('chat.failure.sessionStatus'),
      detail: t('chat.failure.sessionDetail'),
      toast: t('chat.failure.sessionToast'),
    }
  }
  if (
    normalized.includes('embedding') ||
    normalized.includes('milvus') ||
    normalized.includes('compatible embedding') ||
    normalized.includes('vector')
  ) {
    return {
      status: t('chat.failure.retrievalStatus'),
      detail: t('chat.failure.retrievalDetail'),
      toast: t('chat.failure.retrievalToast'),
    }
  }

  return {
    status: t('chat.failure.genericStatus'),
    detail: t('chat.failure.genericDetail'),
    toast: t('chat.failedToast'),
  }
}
