import { streamChat } from '../../api/chat'
import { CHAT_MODEL_OPTIONS } from './models'
import { truncateText } from './utils'

export type ChatFailure = {
  retryable: boolean
  status: string
  detail: string
  toast: string
}

type RetryCallbacks = {
  updateMessage: (sessionID: string, messageID: string, patch: Record<string, unknown>) => void
  appendProcessStep: (sessionID: string, messageID: string, step: string) => void
  appendAnswer: (sessionID: string, messageID: string, content: string) => void
}

type RetryLabels = {
  model: (model: string) => string
  webSearchEnabled: string
  retrying: (attempt: number) => string
  retryNotice: (attempt: number) => string
  rewriteDone: (query: string) => string
  rewriteSkipped: string
  retrievalDone: (returned: number, candidates: number) => string
  rerankDone: string
  rerankSkipped: string
  sourcesDone: (count: number) => string
  webSourcesDone: (count: number) => string
  answerStreaming: string
  finished: string
  retryPrepare: string
  done: string
}

type RetryInput = {
  sessionID: string
  messageID: string
  question: string
  subjectID: string
  llmProvider: string
  llmModel: string
  webSearch: boolean
  topK: number
  maxAskAttempts: number
  askTimeoutMS: number
  labels: RetryLabels
  callbacks: RetryCallbacks
  classifyChatFailure: (error: unknown) => ChatFailure
}

export async function askWithRetry(input: RetryInput) {
  let lastError: unknown

  for (let attempt = 1; attempt <= input.maxAskAttempts; attempt += 1) {
    const controller = new AbortController()
    let completed = false
    let timeout = window.setTimeout(() => controller.abort(), input.askTimeoutMS)
    const resetTimeout = () => {
      window.clearTimeout(timeout)
      timeout = window.setTimeout(() => controller.abort(), input.askTimeoutMS)
    }

    try {
      if (attempt > 1) {
        input.callbacks.updateMessage(input.sessionID, input.messageID, {
          answer: '',
          errorReason: '',
          chunks: [],
          externalLinks: [],
          metrics: undefined,
          processSteps: [
            input.labels.model(
              CHAT_MODEL_OPTIONS.find(
                (option) => option.provider === input.llmProvider && option.model === input.llmModel,
              )?.label ?? input.llmModel,
            ),
            ...(input.webSearch ? [input.labels.webSearchEnabled] : []),
            input.labels.retrying(attempt),
          ],
          status: input.labels.retrying(attempt),
          loading: true,
        })
      }

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
            input.callbacks.updateMessage(input.sessionID, input.messageID, { status })
            input.callbacks.appendProcessStep(input.sessionID, input.messageID, status)
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
          onDone: () => {
            completed = true
            input.callbacks.appendProcessStep(input.sessionID, input.messageID, input.labels.finished)
            input.callbacks.updateMessage(input.sessionID, input.messageID, {
              status: input.labels.done,
              errorReason: '',
              finishedAt: Date.now(),
              loading: false,
            })
          },
          onError: () => {
            input.callbacks.appendProcessStep(input.sessionID, input.messageID, input.labels.retryPrepare)
            input.callbacks.updateMessage(input.sessionID, input.messageID, { status: input.labels.retryPrepare })
          },
        },
        controller.signal,
      )
      window.clearTimeout(timeout)
      return
    } catch (error) {
      window.clearTimeout(timeout)
      if (completed) {
        input.callbacks.updateMessage(input.sessionID, input.messageID, {
          status: input.labels.done,
          errorReason: '',
          finishedAt: Date.now(),
          loading: false,
        })
        return
      }
      lastError = error
      const failure = input.classifyChatFailure(error)
      if (attempt < input.maxAskAttempts && failure.retryable) {
        input.callbacks.updateMessage(input.sessionID, input.messageID, {
          status: input.labels.retryNotice(attempt + 1),
        })
        continue
      }
    }
  }

  throw lastError
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
      retryable: true,
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
      retryable: false,
      status: t('chat.failure.configStatus'),
      detail: t('chat.failure.configDetail'),
      toast: t('chat.failure.configToast'),
    }
  }
  if (normalized.includes('no rows in result set')) {
    return {
      retryable: false,
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
      retryable: false,
      status: t('chat.failure.retrievalStatus'),
      detail: t('chat.failure.retrievalDetail'),
      toast: t('chat.failure.retrievalToast'),
    }
  }

  return {
    retryable: false,
    status: t('chat.failure.genericStatus'),
    detail: t('chat.failure.genericDetail'),
    toast: t('chat.failedToast'),
  }
}
