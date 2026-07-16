import { getStoredToken } from './token'
import type { RetrievalChunk, RetrievalMetrics } from './retrieval'
import { apiClient } from './client'

export interface ExternalLink {
  title: string
  url: string
  snippet: string
}

export interface AgentStep {
  id: string
  kind: string
  title: string
  tool: string
  state: string
  iteration: number
  status: string
  detail: string
  duration_ms: number
}

export interface ChatRun {
  id: string
  status: 'created' | 'running' | 'completed' | 'failed' | 'cancelled'
  subject_id: string
  session_id: string
  message_id: string
  error_message: string
  cancel_requested: boolean
  created_at: string
  updated_at: string
}

export interface ChatRunEvent {
  sequence: number
  type: string
  payload: Record<string, unknown>
  created_at: string
}

export interface StoredChatMessage {
  id: string
  question: string
  answer: string
  chunks: RetrievalChunk[]
  external_links: ExternalLink[]
  metrics: RetrievalMetrics
  model_label: string
  model_id: string
  web_search: boolean
  agent_steps: AgentStep[]
  created_at: string
}

export interface StoredChatSession {
  id: string
  title: string
  subject_id: string
  llm_provider: string
  llm_model: string
  created_at: string
  updated_at: string
  messages: StoredChatMessage[]
}

export async function createChatSession(payload: {
  id: string
  subject_id?: string
  title: string
  llm_provider?: string
  llm_model?: string
}) {
  const response = await apiClient.post<{ session: StoredChatSession }>('/chat/sessions/create', payload)
  return response.data.session
}

export async function listChatSessions() {
  const response = await apiClient.post<{ list: StoredChatSession[] }>('/chat/sessions/list')
  return response.data.list
}

export async function updateChatSession(payload: { id: string; title: string }) {
  await apiClient.post('/chat/sessions/update', payload)
}

export async function deleteChatSession(id: string) {
  await apiClient.post('/chat/sessions/delete', { id })
}

export async function getChatRun(runID: string) {
  const response = await apiClient.post<{
    run: ChatRun
    result: Partial<{
      answer: string
      chunks: RetrievalChunk[]
      external_links: ExternalLink[]
      metrics: RetrievalMetrics
      agent_steps: AgentStep[]
    }>
  }>('/chat/runs/detail', { run_id: runID })
  return response.data
}

export async function listChatRunEvents(runID: string, afterSequence = 0, limit = 200) {
  const response = await apiClient.post<{ list: ChatRunEvent[] }>('/chat/runs/events', {
    run_id: runID,
    after_sequence: afterSequence,
    limit,
  })
  return response.data.list
}

export async function cancelChatRun(runID: string) {
  const response = await apiClient.post<{ cancelled: boolean }>('/chat/runs/cancel', {
    run_id: runID,
  })
  return response.data.cancelled
}

export async function resumeChatRun(runID: string) {
  const response = await apiClient.post<{
    run_id: string
    answer: string
    chunks: RetrievalChunk[]
    external_links: ExternalLink[]
    metrics: RetrievalMetrics
    agent_steps: AgentStep[]
  }>('/chat/runs/resume', { run_id: runID })
  return response.data
}

export interface ChatStreamHandlers {
  onEvent?: () => void
  onRun?: (runID: string) => void
  onStatus?: (message: string) => void
  onAgentStep?: (step: AgentStep) => void
  onSources?: (chunks: RetrievalChunk[]) => void
  onWebSources?: (links: ExternalLink[]) => void
  onMetrics?: (metrics: RetrievalMetrics) => void
  onDelta?: (content: string) => void
  onDone?: (answer: string) => void
  onError?: (message: string) => void
}

export async function streamChat(
  payload: {
    session_id?: string
    message_id?: string
    subject_id: string
    query: string
    top_k?: number
    llm_provider?: string
    llm_model?: string
    web_search?: boolean
    expected_doc_ids?: string[]
    expected_chunk_ids?: string[]
  },
  handlers: ChatStreamHandlers,
  signal?: AbortSignal,
) {
  const response = await fetch('/api/chat/stream', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      Authorization: `Bearer ${getStoredToken()}`,
    },
    body: JSON.stringify(payload),
    signal,
  })

  if (!response.ok || !response.body) {
    const rawBody = await response.text().catch(() => '')
    throw new Error(extractStreamErrorMessage(response.status, rawBody))
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) {
      break
    }

    buffer += decoder.decode(value, { stream: true })
    const events = buffer.split('\n\n')
    buffer = events.pop() ?? ''

    for (const rawEvent of events) {
      if (handleStreamEvent(rawEvent, handlers)) {
        await reader.cancel()
        return
      }
    }
  }

  if (buffer.trim()) {
    handleStreamEvent(buffer, handlers)
  }
}

function extractStreamErrorMessage(status: number, rawBody: string) {
  const body = rawBody.trim()
  if (!body) {
    return `chat_request_failed:${status}`
  }

  try {
    const parsed = JSON.parse(body) as { message?: string; error?: { message?: string } }
    return parsed.message || parsed.error?.message || `chat_request_failed:${status}`
  } catch {
    return body
  }
}

function handleStreamEvent(rawEvent: string, handlers: ChatStreamHandlers) {
  const lines = rawEvent.split('\n')
  const event = lines
    .find((line) => line.startsWith('event:'))
    ?.replace('event:', '')
    .trim()
  const data = lines
    .filter((line) => line.startsWith('data:'))
    .map((line) => line.replace('data:', '').trim())
    .join('')

  if (!event || !data) {
    return false
  }

  handlers.onEvent?.()

  const parsed = JSON.parse(data) as {
    message?: string
    content?: string
    chunks?: RetrievalChunk[]
    links?: ExternalLink[]
    original_query?: string
    search_query?: string
    query_rewritten?: boolean
    reranked?: boolean
    top_k?: number
    candidate_count?: number
    returned_count?: number
    expected_count?: number
    recall_hit_count?: number
    recall_at_k?: number
    id?: string
    kind?: string
    title?: string
    tool?: string
    state?: string
    iteration?: number
    status?: string
    detail?: string
    duration_ms?: number
    answer?: string
    run_id?: string
  }

  if (event === 'run' && parsed.run_id) {
    handlers.onRun?.(parsed.run_id)
    return false
  }

  if (event === 'status' && parsed.message) {
    handlers.onStatus?.(parsed.message)
    return false
  }
  if (event === 'agent_step' && parsed.id && parsed.kind && parsed.title && parsed.status) {
    handlers.onAgentStep?.({
      id: parsed.id,
      kind: parsed.kind,
      title: parsed.title,
      tool: parsed.tool ?? '',
      state: parsed.state ?? '',
      iteration: parsed.iteration ?? 0,
      status: parsed.status,
      detail: parsed.detail ?? '',
      duration_ms: parsed.duration_ms ?? 0,
    })
    return false
  }
  if (event === 'sources') {
    handlers.onSources?.(parsed.chunks ?? [])
    return false
  }
  if (event === 'web_sources') {
    handlers.onWebSources?.(parsed.links ?? [])
    return false
  }
  if (event === 'metrics') {
    handlers.onMetrics?.(parsed as RetrievalMetrics)
    return false
  }
  if (event === 'delta' && parsed.content) {
    handlers.onDelta?.(parsed.content)
    return false
  }
  if (event === 'done') {
    handlers.onDone?.(parsed.answer ?? '')
    return true
  }
  if (event === 'error') {
    handlers.onError?.(parsed.message ?? 'chat_generation_failed')
    throw new Error(parsed.message ?? 'chat_generation_failed')
  }
  return false
}
