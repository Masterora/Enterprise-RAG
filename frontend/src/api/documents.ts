import { apiClient } from './client'

export interface DocumentInfo {
  id: string
  subject_id: string
  filename: string
  file_type: string
  file_size: number
  file_url: string
  status: string
  error_message: string
  progress: number
  created_at: string
  updated_at: string
}

export interface DocumentChunkInfo {
  id: string
  chunk_index: number
  page: number
  section: string
  content: string
  token_count: number
}

export interface IndexTaskInfo {
  id: string
  doc_id: string
  subject_id: string
  filename: string
  task_type: string
  status: string
  retry_count: number
  error_message: string
  created_at: string
  updated_at: string
}

export interface ParseLogInfo {
  id: string
  doc_id: string
  filename: string
  status: string
  message: string
  error_message: string
  created_at: string
}

export interface DocumentDetail {
  document: DocumentInfo
  chunks: DocumentChunkInfo[]
  tasks: IndexTaskInfo[]
  logs: ParseLogInfo[]
}

export async function uploadDocument(payload: { subjectId: string; file: File }) {
  const formData = new FormData()
  formData.append('subject_id', payload.subjectId)
  formData.append('file', payload.file)
  formData.append('processing_mode', 'enhanced')

  const response = await apiClient.post<{ document: DocumentInfo }>('/documents/upload', formData)
  return response.data.document
}

export async function listDocuments(payload: {
  subject_id?: string
  status?: string
  keyword?: string
  page?: number
  page_size?: number
} = {}) {
  const response = await apiClient.post<{ list: DocumentInfo[]; total: number }>('/documents/list', payload)
  return response.data
}

export async function clearFailedDocuments() {
  const response = await apiClient.post<{ deleted: number }>('/documents/clear-failed')
  return response.data.deleted
}

export async function getDocumentDetail(id: string) {
  const response = await apiClient.post<DocumentDetail>('/documents/detail', { id })
  return response.data
}

export async function deleteDocument(id: string) {
  const response = await apiClient.post<{ task: IndexTaskInfo }>('/documents/delete', { id })
  return response.data.task
}

export async function retryIndexTask(id: string) {
  const response = await apiClient.post<{ task: IndexTaskInfo }>('/documents/tasks/retry', { id })
  return response.data.task
}
