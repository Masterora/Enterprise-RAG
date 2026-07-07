import type { ChatSession } from './types'

export const CHAT_STORAGE_KEY = 'enterprise-rag-chat-sessions'

export function loadStoredSessions(): ChatSession[] {
  try {
    const raw = window.localStorage.getItem(CHAT_STORAGE_KEY)
    if (!raw) {
      return []
    }
    const parsed = JSON.parse(raw) as ChatSession[]
    if (!Array.isArray(parsed)) {
      return []
    }
    return parsed
  } catch {
    return []
  }
}

export function buildSessionTitle(question: string) {
  const title = question.trim()
  const runes = [...title]
  if (runes.length <= 18) {
    return title
  }
  return `${runes.slice(0, 18).join('')}...`
}
