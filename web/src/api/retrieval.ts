import { apiClient } from './client'

export interface RetrievalChunk {
  id: string
  evidence_id: string
  doc_id: string
  doc_name: string
  subject_id: string
  user_id: string
  chunk_index: number
  page: number
  section: string
  content: string
  document_version: number
  content_hash: string
  score: number
  raw_score: number
  source: string
}

export interface RetrievalMetrics {
  original_query: string
  search_query: string
  query_rewritten: boolean
  reranked: boolean
  top_k: number
  candidate_count: number
  returned_count: number
  expected_count: number
  recall_hit_count: number
  recall_at_k: number
  route: string
  route_correct: boolean
  latency_ms: number
  evaluation_passed: boolean
  citation_count: number
  answered: boolean
  outcome_correct: boolean
}

export async function searchRetrieval(payload: {
  subject_id: string
  query: string
  top_k?: number
  expected_doc_ids?: string[]
  expected_chunk_ids?: string[]
  expected_route?: string
}) {
  const response = await apiClient.post<{ list: RetrievalChunk[]; metrics: RetrievalMetrics }>('/retrieval/search', payload)
  return response.data
}
