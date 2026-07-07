import { getStoredToken } from './token'
import type { RetrievalChunk } from './retrieval'

export interface ChatStreamHandlers {
  onEvent?: () => void
  onStatus?: (message: string) => void
  onSources?: (chunks: RetrievalChunk[]) => void
  onDelta?: (content: string) => void
  onDone?: () => void
  onError?: (message: string) => void
}

export async function streamChat(
  payload: {
    subject_id: string
    query: string
    top_k?: number
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
    throw new Error('问答请求失败')
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
  }

  if (event === 'status' && parsed.message) {
    handlers.onStatus?.(parsed.message)
    return false
  }
  if (event === 'sources') {
    handlers.onSources?.(parsed.chunks ?? [])
    return false
  }
  if (event === 'delta' && parsed.content) {
    handlers.onDelta?.(parsed.content)
    return false
  }
  if (event === 'done') {
    handlers.onDone?.()
    return true
  }
  if (event === 'error') {
    handlers.onError?.(parsed.message ?? '问答生成失败')
    throw new Error(parsed.message ?? '问答生成失败')
  }
  return false
}
