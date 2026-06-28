import { localeRef, t } from '@/i18n'

const legacyTitleKeys: Record<string, string> = {
  主页: 'app.home',
  演示: 'menu.demo',
}

export function routeTitle(key: string) {
  return () => {
    void localeRef.value
    return t(key)
  }
}

export function resolveTitle(title?: string | Function, fallback = t('common.noTitle')) {
  void localeRef.value
  if (typeof title === 'function') {
    return (title as () => string)()
  }
  if (!title) {
    return fallback
  }
  return t(legacyTitleKeys[title] ?? title)
}

export function translateTreeTitles<T extends { title?: string; i18n?: string; child?: T[] }>(items: T[]): T[] {
  return items.map((item) => {
    const next = {
      ...item,
      title: item.i18n ? t(item.i18n) : item.title ? t(item.title) : item.title,
    }
    if (item.child) {
      next.child = translateTreeTitles(item.child)
    }
    return next
  })
}

export function resolveMenuTitle(title?: string | Function) {
  return resolveTitle(title)
}

export function isNoPermissionMessage(message?: string) {
  return message === t('error.noPermission') || message === '无权访问' || message === 'No permission'
}
