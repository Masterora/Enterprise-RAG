import { apiClient } from './client'
import { clearStoredToken, getStoredToken, setStoredToken } from './token'

export interface UserInfo {
  id: string
  username: string
  email: string
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

export async function login(payload: { username: string; password: string }) {
  const response = await apiClient.post<{ token: string; user: UserInfo }>('/auth/login', payload)
  return response.data
}

export async function getMe() {
  const response = await apiClient.post<{ user: UserInfo }>('/users/me')
  return response.data.user
}
