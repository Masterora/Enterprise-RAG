import type { RetrievalChunk } from '../../api/retrieval'

export type ChatMessage = {
  id: string
  question: string
  answer: string
  status: string
  chunks: RetrievalChunk[]
  loading: boolean
}

export type ChatSession = {
  id: string
  title: string
  subjectID: string
  createdAt: number
  updatedAt: number
  messages: ChatMessage[]
}
