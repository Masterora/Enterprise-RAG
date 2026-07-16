import { apiClient } from './client'

export interface ModelServiceSettings {
  provider: string
  base_url: string
  embedding_model: string
  embedding_dimension: number
  api_key_configured: boolean
  api_key_hint: string
  configuration_source: 'page' | 'environment' | 'none'
}

export async function getModelServiceSettings() {
  const response = await apiClient.post<ModelServiceSettings>('/settings/model-service')
  return response.data
}

export async function updateModelServiceSettings(apiKey: string) {
  const response = await apiClient.post<ModelServiceSettings>(
    '/settings/model-service/update',
    { api_key: apiKey },
    { timeout: 30_000 },
  )
  return response.data
}
