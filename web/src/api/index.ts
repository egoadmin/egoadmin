import axios, { type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'

// import qs from 'qs'
import { ElMessage } from 'element-plus'
import useUserStore from '@/store/modules/user'
import { getAcceptLanguage, t } from '@/i18n'

type AuthRetryConfig = InternalAxiosRequestConfig & { __authRetried?: boolean }

const LOGIN_URL = '/user.v1.UserService/Login'
const LOGOUT_URL = '/user.v1.UserService/Logout'
const AUTH_ERROR_CODES = new Set(['UNAUTHENTICATED', 'LOGIN_EXPIRED', 'NOT_LOGIN'])
const REFRESHABLE_AUTH_ERROR_CODES = new Set(['UNAUTHENTICATED', 'LOGIN_EXPIRED'])
const AUTH_ERROR_MESSAGES = new Set([
  'JWT token is missing',
  'JWT token has expired',
  '未登录',
  '登录已失效',
  '登录已过期',
  '已退出登录',
  '登录已被强制下线',
  '登录已在其他设备生效',
  '登录状态异常，请重新登录',
  '账号不可用',
  'Not logged in',
  'Login has expired',
  'Login is invalid',
  'Logged out',
  'Login has been forced offline',
  'Login has been replaced by another device',
  'Login state is abnormal. Please log in again.',
  'Account is unavailable',
])
const REFRESHABLE_AUTH_ERROR_MESSAGES = new Set([
  'JWT token is missing',
  'JWT token has expired',
  '未登录',
  '登录已失效',
  '登录已过期',
  'Not logged in',
  'Login has expired',
  'Login is invalid',
])
const SILENT_ERROR_MESSAGES = new Set(['JWT token is missing', 'JWT token has expired'])

let activeErrMessage: any = null
let refreshPromise: Promise<any> | null = null
function showErrMessage(msg: string) {
  activeErrMessage?.close()
  activeErrMessage = ElMessage({
    message: msg,
    type: 'error',
  })
}

function isAuthControlRequest(url?: string) {
  return url === LOGIN_URL || url === LOGOUT_URL
}

function isAuthError(responseDate: any) {
  return responseDate?.code === 2 && (AUTH_ERROR_CODES.has(responseDate.reason) || AUTH_ERROR_MESSAGES.has(responseDate.message))
}

function isRefreshableAuthError(responseDate: any) {
  return responseDate?.code === 2 && (REFRESHABLE_AUTH_ERROR_CODES.has(responseDate.reason) || REFRESHABLE_AUTH_ERROR_MESSAGES.has(responseDate.message))
}

function setAuthorizationHeader(config: AuthRetryConfig, token: string) {
  if (!config.headers) {
    return
  }
  config.headers.Authorization = token ? `Bearer ${token}` : ''
}

async function refreshAndRetry(response: AxiosResponse, responseDate: any) {
  const originalConfig = response.config as AuthRetryConfig
  if (
    !isRefreshableAuthError(responseDate) ||
    isAuthControlRequest(originalConfig.url) ||
    originalConfig.__authRetried
  ) {
    return null
  }

  const userStore = useUserStore()
  if (!userStore.refreshToken) {
    return null
  }

  originalConfig.__authRetried = true
  try {
    refreshPromise ??= userStore.refreshLogin().finally(() => {
      refreshPromise = null
    })
    await refreshPromise
    setAuthorizationHeader(originalConfig, userStore.token)
    return api(originalConfig)
  } catch (err) {
    await userStore.logout(undefined, { callServer: false })
    return Promise.reject(err)
  }
}

const api = axios.create({
  baseURL:
    import.meta.env.DEV && import.meta.env.VITE_OPEN_PROXY === 'true'
      ? '/proxy/api'
      : `${window.__APP_CONFIG__?.apiBaseUrl ?? import.meta.env.VITE_APP_API_BASEURL}/api`,
  timeout: 1000 * 600,
  responseType: 'json',
})

api.interceptors.request.use((request) => {
  const userStore = useUserStore()
  /**
   * 全局拦截请求发送前提交的参数
   * 以下代码为示例，在请求头里带上 token 信息
   */
  if (userStore.isLogin && request.headers) {
    request.headers.Authorization = userStore.token ? `Bearer ${userStore.token}` : ''
  }
  if (request.headers) {
    request.headers['Accept-Language'] = getAcceptLanguage()
  }
  // 是否将 POST 请求参数进行字符串化处理
  if (request.method === 'post') {
    // request.data = qs.stringify(request.data, {
    //   arrayFormat: 'brackets',
    // })
  }
  return request
})

api.interceptors.response.use(
  async (response) => {
    /**
     * 全局拦截请求发送后返回的数据，如果数据有报错则在这做全局的错误提示
     * 假设返回数据格式为：{ status: 1, error: '', data: '' }
     * 规则是当 status 为 1 时表示请求成功，为 0 时表示接口需要登录或者登录状态失效，需要重新登录
     * 请求出错时 error 会返回错误信息
     */
    const responseDate: any = response.data || response
    if (responseDate.code) {
      if (responseDate.code === 2) {
        const retryResponse = await refreshAndRetry(response, responseDate)
        if (retryResponse) {
          return retryResponse
        }
        if (isAuthError(responseDate) && !isAuthControlRequest(response.config?.url)) {
          await useUserStore().logout(undefined, { callServer: false })
        }
      }

      if (!SILENT_ERROR_MESSAGES.has(responseDate.message)) {
        // ElMessage.error(responseDate.message)
        showErrMessage(responseDate.message)
      }
      return Promise.reject(responseDate)
    }
    return Promise.resolve(responseDate)
  },
  (error) => {
    let message = error.message
    if (message === 'Network Error') {
      message = t('error.network')
    } else if (message.includes('timeout')) {
      message = t('error.timeout')
    } else if (message.includes('Request failed with status code')) {
      message = t('error.status', { status: message.substr(message.length - 3) })
    }
    ElMessage({
      message,
      type: 'error',
    })
    return Promise.reject(error)
  },
)

export default api
