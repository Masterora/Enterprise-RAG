import axios from 'axios'
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

export async function updateSubject(payload: {
  id: string
  name: string
  description?: string
  visibility?: string
}) {
  const response = await apiClient.post<{ subject: SubjectInfo }>('/subjects/update', payload)
  return response.data.subject
}

export async function deleteSubject(id: string) {
  const response = await apiClient.post<{ deleted: boolean }>('/subjects/delete', { id })
  return response.data.deleted
}

export async function listSubjects(payload: {
  keyword?: string
  page?: number
  page_size?: number
} = {}) {
  const response = await apiClient.post<{ list: SubjectInfo[]; total: number }>('/subjects/list', payload)
  return response.data
}

export function isSubjectNameConflict(error: unknown) {
  return (
    axios.isAxiosError(error) &&
    error.response?.data?.message === 'knowledge base name already exists'
  )
}
