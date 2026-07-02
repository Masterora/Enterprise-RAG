export const TOKEN_STORAGE_KEY = 'enterprise-rag-token'

export function getStoredToken() {
  return window.localStorage.getItem(TOKEN_STORAGE_KEY) ?? ''
}

export function setStoredToken(token: string) {
  window.localStorage.setItem(TOKEN_STORAGE_KEY, token)
}

export function clearStoredToken() {
  window.localStorage.removeItem(TOKEN_STORAGE_KEY)
}
