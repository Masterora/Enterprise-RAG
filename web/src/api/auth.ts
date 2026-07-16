import { apiClient } from './client'
import { clearStoredToken, getStoredToken, setStoredToken } from './token'

export interface UserInfo {
  id: string
  username: string
  nickname: string
  email: string
  language: string
}

export function getAuthToken() {
  return getStoredToken()
}

export function setAuthToken(token: string) {
  setStoredToken(token)
}

export function clearAuthToken() {
  clearStoredToken()
}

export async function login(payload: { username: string; password: string; email?: string }) {
  const response = await apiClient.post<{ token: string; user: UserInfo }>('/auth/login', payload)
  return response.data
}

export async function register(payload: {
  username: string
  password: string
  confirm_password: string
  nickname?: string
  email?: string
}) {
  const response = await apiClient.post<{ token: string; user: UserInfo }>('/auth/register', payload)
  return response.data
}

export async function getMe() {
  const response = await apiClient.post<{ user: UserInfo }>('/users/me')
  return response.data.user
}

export async function updateUser(payload: {
  nickname?: string
  email?: string
  language: string
}) {
  const response = await apiClient.post<{ user: UserInfo }>('/users/update', payload)
  return response.data.user
}

export async function updatePassword(payload: {
  old_password: string
  new_password: string
  confirm_password: string
}) {
  await apiClient.post('/users/password/update', payload)
}
