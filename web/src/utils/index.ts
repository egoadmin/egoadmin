import path from 'path-browserify'

export function resolveRoutePath(basePath: string, routePath?: string) {
  return basePath ? path.resolve(basePath, routePath ?? '') : routePath ?? ''
}

const fileCdnPath = '/cdn/file'
const imageCdnPath = '/cdn/image'
const avatarProcessPath = '120x120/smart'

type ReferenceId = string | number | undefined | null

interface UploadAccessLike {
  referenceId?: ReferenceId
  profile?: string
  contentType?: string
  url?: string
}

function normalizeReferenceId(referenceId?: ReferenceId) {
  const id = `${referenceId ?? ''}`.trim()
  if (!id || id === '0') {
    return ''
  }
  return encodeURIComponent(id)
}

function normalizeProcessPath(processPath?: string) {
  return `${processPath ?? ''}`.replace(/^\/+|\/+$/g, '')
}

function isImageProfile(profile?: string) {
  return profile === 'image' || profile === 'avatar'
}

function isImageContent(contentType?: string) {
  return `${contentType ?? ''}`.toLowerCase().startsWith('image/')
}

export function cdnFileUrl(referenceId?: ReferenceId, options: { display?: string } = {}) {
  const id = normalizeReferenceId(referenceId)
  if (!id) {
    return ''
  }
  const query = options.display ? `?display=${encodeURIComponent(options.display)}` : ''
  return `${fileCdnPath}/${id}${query}`
}

export function cdnImageUrl(referenceId?: ReferenceId, processPath?: string) {
  const id = normalizeReferenceId(referenceId)
  if (!id) {
    return ''
  }
  const normalizedProcessPath = normalizeProcessPath(processPath)
  return `${imageCdnPath}/${id}${normalizedProcessPath ? `/${normalizedProcessPath}` : ''}`
}

export function normalizeGatewayCdnUrl(url?: string) {
  if (!url) {
    return ''
  }
  if (url.startsWith('/cdn/')) {
    return url
  }
  try {
    const parsed = new URL(url, window.location.origin)
    if (parsed.origin === window.location.origin && parsed.pathname.startsWith('/cdn/')) {
      return `${parsed.pathname}${parsed.search}${parsed.hash}`
    }
  } catch {
    return ''
  }
  return ''
}

export function uploadAccessUrl(file?: UploadAccessLike, processPath?: string) {
  const existing = normalizeGatewayCdnUrl(file?.url)
  if (existing) {
    return existing
  }
  if (isImageProfile(file?.profile) || isImageContent(file?.contentType)) {
    return cdnImageUrl(file?.referenceId, processPath)
  }
  return cdnFileUrl(file?.referenceId)
}

export function avatarUrl(user?: { avatar?: string; avatarReferenceId?: string }) {
  if (user?.avatarReferenceId) {
    return cdnImageUrl(user.avatarReferenceId, avatarProcessPath)
  }
  const existing = normalizeGatewayCdnUrl(user?.avatar)
  if (existing) {
    return existing
  }
  const avatar = `${user?.avatar ?? ''}`.trim()
  if (avatar.startsWith('ref-')) {
    return cdnImageUrl(avatar, avatarProcessPath)
  }
  return ''
}
