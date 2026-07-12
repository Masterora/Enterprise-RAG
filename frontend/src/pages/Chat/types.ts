import type { ExternalLink } from '../../api/chat'
import type { RetrievalChunk, RetrievalMetrics } from '../../api/retrieval'

export type ChatMessage = {
  id: string
  question: string
  answer: string
  status: string
  chunks: RetrievalChunk[]
  externalLinks: ExternalLink[]
  metrics?: RetrievalMetrics
  modelLabel?: string
  modelID?: string
  webSearch?: boolean
  processSteps: string[]
  startedAt?: number
  finishedAt?: number
  errorReason?: string
  loading: boolean
}

export type ChatSession = {
  id: string
  title: string
  subjectID: string
  llmProvider: string
  llmModel: string
  createdAt: number
  updatedAt: number
  messages: ChatMessage[]
}
