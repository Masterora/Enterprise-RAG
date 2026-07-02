import { apiClient } from './client'

export interface RetrievalChunk {
  id: string
  doc_id: string
  doc_name: string
  subject_id: string
  user_id: string
  chunk_index: number
  page: number
  section: string
  content: string
  score: number
}

export async function searchRetrieval(payload: {
  subject_id: string
  query: string
  top_k?: number
}) {
  const response = await apiClient.post<{ list: RetrievalChunk[] }>('/retrieval/search', payload)
  return response.data.list
}
