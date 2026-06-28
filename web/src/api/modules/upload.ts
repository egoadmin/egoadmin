import api from '../index'
import useUserStore from '@/store/modules/user'
import { getAcceptLanguage } from '@/i18n'
import { uploadAccessUrl } from '@/utils'
import * as tus from 'tus-js-client'

const uploadBaseURL =
  import.meta.env.DEV && import.meta.env.VITE_OPEN_PROXY === 'true'
    ? '/proxy'
    : (window.__APP_CONFIG__?.apiBaseUrl ?? import.meta.env.VITE_APP_API_BASEURL ?? '')

export interface UploadProfile {
  name: string
  maxSize: number
  ttlSeconds: number
  allowedExtensions?: string[]
  allowedMimeTypes?: string[]
  tusRequired: boolean
  maxCount?: number
  instantEnabled: boolean
}

export interface UploadFileOptions {
  profile?: string
  file: File
  sha256?: string
  preferTus?: boolean
  onProgress?: (percent: number) => void
}

export interface UploadedFile {
  filename?: string
  originame?: string
  size?: string
  fileId: string
  referenceId: string
  profile: string
  contentType?: string
  status?: string
  expiresAt?: string
  url?: string
}

export interface InstantUploadResult {
  hit: boolean
  shouldUpload: boolean
  fileId?: string
  referenceId?: string
  profile?: string
  url?: string
  expiresAt?: string
}

interface TusUploadMetadata {
  fileId: string
  referenceId: string
  profile: string
  contentType?: string
  status?: string
  expiresAt?: string
  url?: string
}

class UploadRequestError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'UploadRequestError'
    this.status = status
  }
}

function uploadURL(path: string) {
  return `${uploadBaseURL}${path}`
}

function absoluteUploadURL(path: string) {
  if (path.startsWith('http')) {
    return path
  }
  return uploadURL(path)
}

function tusMetadataStorageKey(url: string) {
  return `egoadmin:tus:${url}`
}

function saveTusUploadMetadata(url: string | null | undefined, metadata: TusUploadMetadata) {
  if (!url) {
    return
  }
  localStorage.setItem(tusMetadataStorageKey(url), JSON.stringify(metadata))
}

function loadTusUploadMetadata(url: string | null | undefined) {
  if (!url) {
    return null
  }
  const raw = localStorage.getItem(tusMetadataStorageKey(url))
  if (!raw) {
    return null
  }
  try {
    return JSON.parse(raw) as TusUploadMetadata
  } catch {
    localStorage.removeItem(tusMetadataStorageKey(url))
    return null
  }
}

function removeTusUploadMetadata(url: string | null | undefined) {
  if (!url) {
    return
  }
  localStorage.removeItem(tusMetadataStorageKey(url))
}

function authHeaders() {
  const userStore = useUserStore()
  return {
    ...(userStore.token ? { Authorization: `Bearer ${userStore.token}` } : {}),
    'Accept-Language': getAcceptLanguage(),
  }
}

function parseUploadResponse(res: any): UploadedFile {
  const file = res?.files?.[0] ?? res
  const parsed = {
    ...file,
    fileId: `${file.fileId ?? ''}`,
    referenceId: `${file.referenceId ?? ''}`,
  } satisfies UploadedFile
  return {
    ...parsed,
    url: uploadAccessUrl(parsed),
  }
}

function isUploadAuthError(error: unknown) {
  if (error instanceof UploadRequestError && error.status === 401) {
    return true
  }
  return error instanceof tus.DetailedError && error.originalResponse?.getStatus() === 401
}

async function refreshUploadAuth() {
  const userStore = useUserStore()
  if (!userStore.refreshToken) {
    throw new UploadRequestError(401, 'upload authorization expired')
  }
  try {
    await userStore.refreshLogin()
  } catch (err) {
    await userStore.logout(undefined, { callServer: false })
    throw err
  }
}

async function uploadMultipart(options: UploadFileOptions) {
  const { file, profile = 'default', sha256, onProgress } = options
  const formData = new FormData()
  formData.append('json', JSON.stringify([{
    name: file.name,
    size: `${file.size}`,
    profile,
    contentType: file.type,
    sha256,
  }]))
  formData.append('file', file)
  const res = await api.post('/upload', formData, {
    baseURL: uploadBaseURL,
    onUploadProgress: (event) => {
      if (event.total && onProgress) {
        onProgress(Math.round((event.loaded / event.total) * 100))
      }
    },
  })
  return parseUploadResponse(res)
}

async function uploadTus(options: UploadFileOptions) {
  const { file, profile = 'default', sha256, onProgress } = options
  const result: TusUploadMetadata = {
    fileId: '',
    referenceId: '',
    profile,
    contentType: file.type,
  }
  let refreshed = false
  let refreshPromise: Promise<void> | null = null
  let resumedUploadURL: string | null = null
  const upload = new tus.Upload(file, {
    endpoint: uploadURL('/tus/upload'),
    metadata: {
      filename: file.name,
      profile,
      filetype: file.type,
      contentType: file.type,
      ...(sha256 ? { sha256 } : {}),
    },
    headers: authHeaders(),
    retryDelays: [0, 1000, 3000, 5000],
    removeFingerprintOnSuccess: true,
    onBeforeRequest: async (req) => {
      if (refreshPromise) {
        await refreshPromise
      }
      Object.entries(authHeaders()).forEach(([key, value]) => req.setHeader(key, value))
    },
    onAfterResponse: (_req, res) => {
      if (res.getStatus() === 201) {
        result.fileId = res.getHeader('X-Upload-File-Id') || result.fileId
        result.referenceId = res.getHeader('X-Upload-Reference-Id') || result.referenceId
        result.profile = res.getHeader('X-Upload-Profile') || result.profile
        result.status = res.getHeader('X-Upload-Status') || result.status
        result.expiresAt = res.getHeader('X-Upload-Expires-At') || result.expiresAt
        result.url = res.getHeader('X-Upload-Url') || result.url
        const location = res.getHeader('Location')
        saveTusUploadMetadata(location, result)
        if (location) {
          saveTusUploadMetadata(absoluteUploadURL(location), result)
        }
        saveTusUploadMetadata(upload.url, result)
      }
    },
    onShouldRetry: (err, retryAttempt, tusOptions) => {
      if (!isUploadAuthError(err)) {
        return tus.defaultOptions.onShouldRetry?.(err, retryAttempt, tusOptions) ?? retryAttempt < (tusOptions.retryDelays?.length ?? 0)
      }
      if (refreshed || !useUserStore().refreshToken) {
        return false
      }
      refreshed = true
      refreshPromise = refreshUploadAuth().finally(() => {
        refreshPromise = null
      })
      return true
    },
    onProgress: (bytesSent, bytesTotal) => {
      if (bytesTotal > 0 && onProgress) {
        onProgress(Math.round((bytesSent / bytesTotal) * 100))
      }
    },
  })
  const previousUploads = await upload.findPreviousUploads()
  if (previousUploads.length > 0) {
    const previousUpload = previousUploads[0]
    upload.resumeFromPreviousUpload(previousUpload)
    resumedUploadURL = previousUpload.uploadUrl
    const metadata = loadTusUploadMetadata(previousUpload.uploadUrl)
    if (metadata) {
      Object.assign(result, metadata)
    }
  }

  await new Promise<void>((resolve, reject) => {
    upload.options.onSuccess = () => resolve()
    upload.options.onError = (error) => reject(error)
    upload.start()
  })
  Object.assign(result, loadTusUploadMetadata(upload.url) ?? {})
  removeTusUploadMetadata(upload.url)
  removeTusUploadMetadata(resumedUploadURL)

  const uploaded = {
    fileId: result.fileId,
    referenceId: result.referenceId,
    profile: result.profile,
    contentType: result.contentType || file.type,
    filename: file.name,
    originame: file.name,
    size: `${file.size}`,
    status: result.status,
    expiresAt: result.expiresAt,
    url: result.url,
  } satisfies UploadedFile
  return {
    ...uploaded,
    url: uploadAccessUrl(uploaded),
  }
}

async function uploadFile(options: UploadFileOptions) {
  const { file, profile = 'default', sha256, preferTus = false } = options
  if (sha256) {
    const instant: InstantUploadResult = await api.post('/upload/instant', {
      profile,
      sha256,
      size: file.size,
      filename: file.name,
      contentType: file.type,
    }, { baseURL: uploadBaseURL })
    if (instant.hit && !instant.shouldUpload) {
      const uploaded = {
        fileId: instant.fileId ?? '0',
        referenceId: instant.referenceId ?? '0',
        profile: instant.profile ?? profile,
        contentType: file.type,
        filename: file.name,
        originame: file.name,
        size: `${file.size}`,
        expiresAt: instant.expiresAt,
        url: instant.url,
      } satisfies UploadedFile
      return {
        ...uploaded,
        url: uploadAccessUrl(uploaded),
      }
    }
  }

  const profiles = await getProfiles()
  const uploadProfile = profiles.find(item => item.name === profile)
  if (preferTus || uploadProfile?.tusRequired) {
    return uploadTus(options)
  }
  return uploadMultipart(options)
}

async function getProfiles() {
  const res: any = await api.get('/upload/profiles', { baseURL: uploadBaseURL })
  return (res?.profiles ?? []) as UploadProfile[]
}

export default {
  upload: (data: FormData) => api.post('/upload', data, { baseURL: uploadBaseURL }),
  getProfiles,
  instant: (data: {
    profile?: string
    sha256: string
    size: number
    filename: string
    contentType?: string
  }) => api.post('/upload/instant', data, { baseURL: uploadBaseURL }),
  uploadFile,
  uploadMultipart,
  uploadTus,
}
