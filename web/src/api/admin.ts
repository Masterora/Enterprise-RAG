import { apiClient } from './client'
import type { IndexTaskInfo, ParseLogInfo } from './documents'

export interface AdminSummary {
  subject_total: number
  document_total: number
  chunk_total: number
  session_total: number
  indexed_total: number
  processing_total: number
  failed_total: number
  pending_task_total: number
  running_task_total: number
  failed_task_total: number
}

export async function getAdminSummary() {
  const response = await apiClient.post<AdminSummary>('/admin/summary')
  return response.data
}

export async function listAdminTasks(payload: {
  subject_id?: string
  status?: string
  task_type?: string
  page?: number
  page_size?: number
} = {}) {
  const response = await apiClient.post<{ list: IndexTaskInfo[]; total: number }>(
    '/admin/tasks/list',
    payload,
  )
  return response.data
}

export async function clearAdminTasks() {
  const response = await apiClient.post<{ cleared: number }>('/admin/tasks/clear')
  return response.data.cleared
}

export async function clearAdminLogs(payload: { subject_id?: string } = {}) {
  const response = await apiClient.post<{ cleared: number }>('/admin/logs/clear', payload)
  return response.data.cleared
}

export async function listAdminLogs(payload: {
  subject_id?: string
  status?: string
  page?: number
  page_size?: number
} = {}) {
  const response = await apiClient.post<{ list: ParseLogInfo[]; total: number }>(
    '/admin/logs/list',
    payload,
  )
  return response.data
}
