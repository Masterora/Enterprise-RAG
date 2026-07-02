import axios from 'axios'
import { getStoredToken } from './token'

export const apiClient = axios.create({
  baseURL: '/api',
  timeout: 15_000,
})

apiClient.interceptors.request.use((config) => {
  const token = getStoredToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})
