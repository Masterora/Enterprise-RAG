import { createContext } from 'react'
import { enUS } from './i18n/en-US'
import { jaJP } from './i18n/ja-JP'
import { zhCN } from './i18n/zh-CN'

export const LANGUAGE_STORAGE_KEY = 'enterprise-rag-language'

export type Language = 'zh-CN' | 'en-US' | 'ja-JP'

export const LANGUAGE_LABELS: Record<Language, string> = {
  'zh-CN': '中文',
  'en-US': 'English',
  'ja-JP': '日本語',
}

type Dictionary = Record<string, string>

export const dictionaries: Record<Language, Dictionary> = {
  'zh-CN': zhCN,
  'en-US': enUS,
  'ja-JP': jaJP,
}

export type I18nContextValue = {
  language: Language
  setLanguage: (language: Language) => void
  t: (key: string, params?: Record<string, string | number>) => string
}

export const I18nContext = createContext<I18nContextValue | null>(null)

export function isSupportedLanguage(value: string): value is Language {
  return value === 'zh-CN' || value === 'en-US' || value === 'ja-JP'
}

export function readStoredLanguage(): Language {
  const stored = window.localStorage.getItem(LANGUAGE_STORAGE_KEY)
  return stored && isSupportedLanguage(stored) ? stored : 'zh-CN'
}

export function translate(
  language: Language,
  key: string,
  params?: Record<string, string | number>,
) {
  const template = dictionaries[language][key] ?? dictionaries['zh-CN'][key] ?? key
  if (!params) {
    return template
  }
  return template.replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? ''))
}
