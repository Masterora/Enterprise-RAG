import { apiClient } from './client'

export interface DocumentInfo {
  id: string
  subject_id: string
  filename: string
  file_type: string
  file_size: number
  file_url: string
  status: string
  created_at: string
  updated_at: string
}

export async function uploadDocument(payload: { subjectId: string; file: File }) {
  const formData = new FormData()
  formData.append('subject_id', payload.subjectId)
  formData.append('file', payload.file)

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
