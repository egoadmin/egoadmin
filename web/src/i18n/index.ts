import { createI18n } from 'vue-i18n'
import zhCN from './package/zh-CN'
import enUS from './package/en-US'

export type LocaleKey = 'zh-CN' | 'en'
export const localeChangedEvent = 'egoadmin-locale-changed'

export const localeOptions: Array<{
  value: LocaleKey
  label: string
  acceptLanguage: string
}> = [
  { value: 'zh-CN', label: '简体中文', acceptLanguage: 'zh-CN' },
  { value: 'en', label: 'English', acceptLanguage: 'en-US' },
]

const storageKey = 'egoadmin-locale'

function normalizeLocale(value?: string | null): LocaleKey {
  if (!value) {
    return 'zh-CN'
  }
  const lower = value.toLowerCase()
  if (lower === 'en' || lower.startsWith('en-')) {
    return 'en'
  }
  return 'zh-CN'
}

const initialLocale = normalizeLocale(localStorage.getItem(storageKey) ?? navigator.language)
localStorage.setItem(storageKey, initialLocale)

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: initialLocale,
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    en: enUS,
  },
})

export const localeRef = i18n.global.locale

export function getLocale() {
  return localeRef.value as LocaleKey
}

export function setLocale(locale: LocaleKey) {
  localeRef.value = locale
  localStorage.setItem(storageKey, locale)
  window.dispatchEvent(new CustomEvent(localeChangedEvent, { detail: locale }))
}

export function getAcceptLanguage() {
  return localeOptions.find(item => item.value === getLocale())?.acceptLanguage ?? 'zh-CN'
}

export function t(key: string, data?: Record<string, unknown>) {
  void localeRef.value
  return (i18n.global.t as (key: string, data?: Record<string, unknown>) => string)(key, data)
}

export default i18n
