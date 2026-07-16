import { useContext } from 'react'
import { I18nContext } from './i18n-core'

export { type Language } from './i18n-core'

export function useI18n() {
  const context = useContext(I18nContext)
  if (!context) {
    throw new Error('I18nProvider is required')
  }
  return context
}
