import useRouteStore from './route'
import useMenuStore from './menu'
import router, { resetRouter } from '@/router'
import apiUser from '@/api/modules/user'
import {
  broadcastHeartbeatLogout,
  broadcastHeartbeatTokenUpdated,
  startHeartbeat,
  stopHeartbeat,
} from '@/utils/heartbeat'
import { LOGIN_CRYPTO_ACTION, encryptPasswordPayload } from '@/utils/login-crypto'

const useUserStore = defineStore(
  // 唯一ID
  'user',
  () => {
    const routeStore = useRouteStore()
    const menuStore = useMenuStore()
    const token = ref(localStorage.getItem('token') ?? '')
    const refreshToken = ref(localStorage.getItem('refreshToken') ?? '')
    const userInfo: any = ref({})
    const username: any = ref(localStorage.getItem('username') ?? '')
    const permissions = ref<string[]>([])
    const avatar = ref(localStorage.getItem('avatar') ?? '')
    const menus = ref(localStorage.getItem('menus') ?? '')
    const isLogin = computed(() => {
      let retn = false
      if (token.value) {
        retn = true
      }
      return retn
    })
    function syncTokenFromStorage() {
      token.value = localStorage.getItem('token') ?? ''
      refreshToken.value = localStorage.getItem('refreshToken') ?? ''
      username.value = localStorage.getItem('username') ?? ''
      avatar.value = localStorage.getItem('avatar') ?? ''
      menus.value = localStorage.getItem('menus') ?? ''
    }
    function clearLocalLoginState() {
      localStorage.removeItem('token')
      localStorage.removeItem('refreshToken')
      localStorage.removeItem('menus')
      localStorage.removeItem('username')
      resetRouter()
      token.value = ''
      refreshToken.value = ''
      userInfo.value = ''
      avatar.value = ''
      menus.value = ''
      permissions.value = []
      routeStore.removeRoutes()
      menuStore.setActived(0)
    }
    function startHeartbeatReporting() {
      startHeartbeat({
        hasToken: () => !!token.value,
        getToken: () => token.value,
        heartbeat: () => apiUser.heartBeatUser(),
        onLogout: () => {
          void logout(undefined, { callServer: false, broadcast: false })
        },
        onTokenUpdated: () => {
          syncTokenFromStorage()
          if (token.value) {
            startHeartbeatReporting()
          }
        },
        offlineOnPageLeave: () => window.__APP_CONFIG__?.offlineOnPageLeave === true,
      })
    }
    // 登录
    async function login(data: {
      username?: string
      password?: string
      ua?: string
      token?: string
    }) {
      let loginData: any = data
      if (!data.token) {
        if (!data.username || !data.password || !data.ua) {
          throw new Error('login data is incomplete')
        }
        const encrypted = await encryptPasswordPayload({
          username: data.username,
          password: data.password,
          ua: data.ua,
          action: LOGIN_CRYPTO_ACTION.login,
        })
        loginData = {
          username: data.username,
          ua: data.ua,
          ...encrypted,
        }
      }
      const res: any = await apiUser.login(loginData)
      token.value = res.token
      refreshToken.value = res.refreshToken ?? ''
      // menus.value = res.menus
      localStorage.setItem('token', res.token)
      localStorage.setItem('refreshToken', refreshToken.value)
      broadcastHeartbeatTokenUpdated()
      // localStorage.setItem('menus', res.menus)
      await getUserInfo()
      localStorage.setItem('username', userInfo.value.username)
      username.value = userInfo.value.username
      startHeartbeatReporting()
    }
    // 刷新登录态
    async function refreshLogin() {
      if (!refreshToken.value) {
        throw new Error('refresh token is missing')
      }
      const res: any = await apiUser.login({ token: refreshToken.value })
      token.value = res.token
      refreshToken.value = res.refreshToken ?? ''
      localStorage.setItem('token', res.token)
      localStorage.setItem('refreshToken', refreshToken.value)
      broadcastHeartbeatTokenUpdated()
      startHeartbeatReporting()
      return res
    }
    // 登出
    async function logout(
      redirect = router.currentRoute.value.fullPath,
      options: { callServer?: boolean; broadcast?: boolean } = {},
    ) {
      const { callServer = true, broadcast = true } = options
      if (callServer && token.value) {
        await apiUser.logout().catch(() => {})
      }
      if (broadcast) {
        broadcastHeartbeatLogout()
      } else {
        stopHeartbeat()
      }
      clearLocalLoginState()

      void router.push({
        name: 'login',
        query: {
          ...(router.currentRoute.value.path !== '/' &&
            router.currentRoute.value.name !== 'login' && { redirect }),
        },
      })
    }
    // 获取用户信息
    async function getUserInfo() {
      const info: any = await apiUser.getCenterInfo()
      userInfo.value = info.user
      return userInfo.value
    }
    // 获取我的权限
    async function getPermissions() {
      // 通过 mock 获取权限
      const res: any = await apiUser.getMenus()
      // console.log(res.menus, '11111')
      localStorage.setItem('menus', res.menus)
      menus.value = res.menus
      // permissions.value = res.menus
      return menus.value ? menus.value.split(',') : []
    }
    // 恢复登录态与心跳
    async function bootstrapSession() {
      if (!token.value) {
        stopHeartbeat()
        return
      }
      startHeartbeatReporting()
      try {
        await getUserInfo()
      } catch {
        if (!refreshToken.value) {
          await logout(undefined, { callServer: false })
        }
      }
    }
    // 心跳
    function heartBeatUser() {
      if (token.value) {
        startHeartbeatReporting()
      } else {
        stopHeartbeat()
      }
    }
    // 权限验证
    function VA(ids: string[]) {
      const roleList = menus.value.split(',')
      if (username.value === 'root' || username.value === 'admin') {
        return true
      }
      return roleList.some((item: any) => ids.some((row: any) => `${item}` === `${row}`))
    }
    return {
      token,
      refreshToken,
      permissions,
      avatar,
      isLogin,
      userInfo,
      username,
      menus,
      login,
      refreshLogin,
      logout,
      bootstrapSession,
      getPermissions,
      getUserInfo,
      heartBeatUser,
      VA,
    }
  },
)

export default useUserStore
