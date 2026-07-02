import { apiClient } from './client'

export interface SubjectInfo {
  id: string
  name: string
  description: string
  visibility: string
  created_at: string
  updated_at: string
}

export async function createSubject(payload: {
  name: string
  description?: string
  visibility?: string
}) {
  const response = await apiClient.post<{ subject: SubjectInfo }>('/subjects/create', payload)
  return response.data.subject
}

export async function listSubjects(payload: {
  keyword?: string
  page?: number
  page_size?: number
} = {}) {
  const response = await apiClient.post<{ list: SubjectInfo[]; total: number }>('/subjects/list', payload)
  return response.data
}
