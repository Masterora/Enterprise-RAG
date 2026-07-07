import { useMemo, useState, type ReactNode } from 'react'
import {
  I18nContext,
  LANGUAGE_STORAGE_KEY,
  readStoredLanguage,
  translate,
  type I18nContextValue,
} from './i18n-core'

export function I18nProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState(readStoredLanguage)

  const value = useMemo<I18nContextValue>(
    () => ({
      language,
      setLanguage: (nextLanguage) => {
        window.localStorage.setItem(LANGUAGE_STORAGE_KEY, nextLanguage)
        setLanguageState(nextLanguage)
      },
      t: (key, params) => translate(language, key, params),
    }),
    [language],
  )

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}
