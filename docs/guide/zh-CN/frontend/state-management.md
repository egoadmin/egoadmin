# 状态管理

EgoAdmin 使用 Pinia 进行全局状态管理，按业务领域拆分为多个独立模块，配合组合式函数实现可复用的业务逻辑。

## 概述

项目采用**单 Pinia 实例 + 多模块**架构，所有 store 位于 `store/modules/` 目录下：

```text
web/src/store/modules/
├── user.ts          # 用户状态：登录、Token、权限、会话
├── permission.ts    # 权限路由：动态路由生成与过滤
├── route.ts         # 路由状态：已注册路由与系统路由
├── menu.ts          # 菜单状态：导航树、选中态、面包屑
├── keepAlive.ts     # 缓存状态：页面级组件缓存列表
└── settings.ts      # 设置状态：主题、语言、布局偏好
```

每个 store 使用 Composition API 风格（`defineStore` + `setup` 函数），与 Vue 3 `<script setup>` 风格一致。

## 核心用法

### User Store（用户状态）

用户 store 是整个认证体系的核心，管理登录态、Token、用户信息和权限校验：

```typescript
// store/modules/user.ts
import useRouteStore from './route'
import useMenuStore from './menu'
import router, { resetRouter } from '@/router'
import apiUser from '@/api/modules/user'

const useUserStore = defineStore('user', () => {
  // ---------- 状态 ----------
  const token = ref(localStorage.getItem('token') ?? '')
  const refreshToken = ref(localStorage.getItem('refreshToken') ?? '')
  const userInfo: any = ref({})
  const username: any = ref(localStorage.getItem('username') ?? '')
  const permissions = ref<string[]>([])
  const menus = ref(localStorage.getItem('menus') ?? '')

  const isLogin = computed(() => !!token.value)

  // ---------- 登录 ----------
  async function login(data: {
    username?: string
    password?: string
    ua?: string
    token?: string
  }) {
    let loginData: any = data
    if (!data.token) {
      // 使用 LoginCrypto 加密密码
      const encrypted = await encryptPasswordPayload({
        username: data.username!,
        password: data.password!,
        ua: data.ua!,
        action: LOGIN_CRYPTO_ACTION.login,
      })
      loginData = { username: data.username, ua: data.ua, ...encrypted }
    }
    const res: any = await apiUser.login(loginData)
    token.value = res.token
    refreshToken.value = res.refreshToken ?? ''
    localStorage.setItem('token', res.token)
    localStorage.setItem('refreshToken', refreshToken.value)
    await getUserInfo()
    localStorage.setItem('username', userInfo.value.username)
    username.value = userInfo.value.username
    startHeartbeatReporting()
  }

  // ---------- 登出 ----------
  async function logout(
    redirect = router.currentRoute.value.fullPath,
    options: { callServer?: boolean; broadcast?: boolean } = {},
  ) {
    const { callServer = true, broadcast = true } = options
    if (callServer && token.value) {
      await apiUser.logout().catch(() => {})
    }
    clearLocalLoginState()
    void router.push({
      name: 'login',
      query: { ...(redirect && { redirect }) },
    })
  }

  // ---------- 权限校验 ----------
  function VA(ids: string[]) {
    const roleList = menus.value.split(',')
    if (username.value === 'root' || username.value === 'admin') {
      return true
    }
    return roleList.some((item: any) =>
      ids.some((row: any) => `${item}` === `${row}`)
    )
  }

  return {
    token, refreshToken, permissions, userInfo, username, menus, isLogin,
    login, logout, getUserInfo, getPermissions, VA,
  }
})

export default useUserStore
```

`VA` 方法用于按钮级权限校验，接收权限 ID 数组，返回当前用户是否拥有其中任意一个权限。

### Permission Store（权限路由）

负责根据用户权限过滤并生成动态路由：

```typescript
// store/modules/permission.ts
import type { RouteRecordRaw } from 'vue-router'
import useUserStore from './user'
import { asyncRoutes } from '@/router/routes'
import { menu } from '@/config/routeMenu'

const usePermissionStore = defineStore('permission', () => {
  const userStore = useUserStore()
  const routes = ref([])
  const dynamicRoutes = ref([])

  function hasPermission(roles: string[], routes: RouteRecordRaw | any) {
    // 对路由做权限匹配，返回允许的子路由
    const data: any = []
    const route = [...routes[0].children]
    handleRoutes(route)
    route.forEach((item: any) => {
      roles.forEach((val: any) => {
        if (val === item.path) {
          data.push(item)
        }
      })
    })
    return data
  }

  function filterAsyncRoutes(routes: any, roles: any) {
    // 递归提取权限匹配的路径
    const res: string[] = []
    const filterPath = (childs: any) => {
      childs.forEach((item: any) => {
        roles?.forEach((data: string) => {
          if (`${data}` === `${item.id}` && item.path) {
            res.push(item.path)
          }
        })
        if (item.child) filterPath(item.child)
      })
    }
    filterPath(routes)
    // 确保父路径也被包含
    routes.forEach((item: any) => {
      const possessorId = item.child?.map((row: any) => `${row.path}`) || []
      const parentPath = item.path
      res.forEach((data: any) => {
        if (possessorId.includes(data)) {
          res.push(parentPath)
        }
      })
    })
    const pathList = Array.from(new Set(res))
    return hasPermission(pathList, asyncRoutes)
  }

  async function setRoutes(roles: any) {
    let accessedRoutes
    const rolesArr = typeof roles !== 'object' ? roles.split(',') : roles

    if (userStore.username === 'admin' || userStore.username === 'root') {
      accessedRoutes = asyncRoutes[0].children
    } else {
      accessedRoutes = await filterAsyncRoutes(menu, rolesArr)
    }
    dynamicRoutes.value = cloneDeep(accessedRoutes)
  }

  return { routes, dynamicRoutes, setRoutes, hasPermission, filterAsyncRoutes }
})

export default usePermissionStore
```

::: warning admin/root 跳过过滤
`admin` 和 `root` 用户直接拥有所有路由，不做权限过滤。生产环境请确保这些账号的安全性。
:::

### Menu Store（菜单状态）

管理导航菜单树、主导航选中态和面包屑：

```typescript
// store/modules/menu.ts
const useMenuStore = defineStore('menu', () => {
  const settingsStore = useSettingsStore()
  const routeStore = useRouteStore()

  const menus = ref<Menu.recordMainRaw[]>([{ meta: {}, children: [] }])
  const actived = ref(0)

  // 完整导航数据
  const allMenus = computed(() => {
    let returnMenus: Menu.recordMainRaw[] = [{ meta: {}, children: [] }]
    if (settingsStore.settings.app.routeBaseOn !== 'filesystem') {
      if (settingsStore.settings.menu.menuMode === 'single') {
        // 单导航模式：合并所有顶级路由的 children
        returnMenus[0].children = []
        routeStore.routes.forEach((item) => {
          returnMenus[0].children?.push(...(item.children as Menu.recordRaw[]))
        })
      } else {
        returnMenus = routeStore.routes as Menu.recordMainRaw[]
      }
    }
    return returnMenus
  })

  // 次导航数据（侧边栏）
  const sidebarMenus = computed(() => {
    return allMenus.value.length > 0
      ? allMenus.value[actived.value].children
      : []
  })

  // 切换主导航
  function setActived(data: number | string) {
    if (typeof data === 'number') {
      actived.value = data
    } else {
      // 根据路由路径定位主导航索引
      const findIndex = allMenus.value.findIndex((item) =>
        item.children.some(
          (r) => data.indexOf(`${r.path}/`) === 0 || data === r.path,
        ),
      )
      if (findIndex >= 0) actived.value = findIndex
    }
  }

  return { menus, actived, allMenus, sidebarMenus, setActived }
})

export default useMenuStore
```

### KeepAlive Store（缓存状态）

管理页面级组件缓存列表，与 Vue 的 `<KeepAlive>` 配合使用：

```typescript
// store/modules/keepAlive.ts
const useKeepAliveStore = defineStore('keepAlive', () => {
  const list = ref<string[]>([])

  function add(name: string | string[]) {
    if (typeof name === 'string') {
      !list.value.includes(name) && list.value.push(name)
    } else {
      name.forEach((v) => {
        v && !list.value.includes(v) && list.value.push(v)
      })
    }
  }

  function remove(name: string | string[]) {
    if (typeof name === 'string') {
      list.value = list.value.filter((v) => v !== name)
    } else {
      list.value = list.value.filter((v) => !name.includes(v))
    }
  }

  function clean() {
    list.value = []
  }

  return { list, add, remove, clean }
})

export default useKeepAliveStore
```

在布局组件中使用：

```vue
<template>
  <router-view v-slot="{ Component }">
    <keep-alive :include="keepAliveStore.list">
      <component :is="Component" />
    </keep-alive>
  </router-view>
</template>
```

### Settings Store（设置状态）

管理全局设置，包括主题、布局和用户偏好：

```typescript
// store/modules/settings.ts
import { defaultsDeep } from 'lodash-es'
import settingsCustom from '@/settings'
import settingsDefault from '@/settings.default'

const useSettingsStore = defineStore('settings', () => {
  const mergeSettings = defaultsDeep(settingsCustom, settingsDefault)
  const settings = ref(mergeSettings)

  // 主题色切换：监听 colorScheme 变化，操作根元素 class
  watch(
    () => settings.value.app.colorScheme,
    (val) => {
      if (val === '') {
        val = window.matchMedia('(prefers-color-scheme: dark)').matches
          ? 'dark'
          : 'light'
      }
      switch (val) {
        case 'dark':
          document.documentElement.classList.add('dark')
          break
        case 'light':
          document.documentElement.classList.remove('dark')
          break
      }
    },
    { immediate: true },
  )

  // 移动端适配
  const mode = ref<'pc' | 'mobile'>('pc')
  function setMode(width: number) {
    if (settings.value.layout.enableMobileAdaptation) {
      if (/Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i
        .test(navigator.userAgent)) {
        mode.value = 'mobile'
      } else {
        mode.value = width < 992 ? 'mobile' : 'pc'
      }
    }
  }

  // 侧边栏切换
  function toggleSidebarCollapse() {
    settings.value.menu.subMenuCollapse = !settings.value.menu.subMenuCollapse
  }

  // 页面标题
  const title = ref<RouteMeta['title']>()
  function setTitle(_title: RouteMeta['title']) {
    title.value = _title
  }

  return {
    settings, mode, title, setTitle, setMode, toggleSidebarCollapse,
  }
})

export default useSettingsStore
```

## 配置示例

### 权限校验在模板中的使用

```vue
<template>
  <div>
    <!-- 按钮级权限：仅对有权限的用户显示 -->
    <el-button
      v-if="userStore.VA(['sys:user:add'])"
      type="primary"
      @click="handleAdd"
    >
      新增用户
    </el-button>

    <el-button
      v-if="userStore.VA(['sys:user:edit'])"
      @click="handleEdit"
    >
      编辑
    </el-button>

    <el-button
      v-if="userStore.VA(['sys:user:delete'])"
      type="danger"
      @click="handleDelete"
    >
      删除
    </el-button>
  </div>
</template>

<script setup lang="ts">
import useUserStore from '@/store/modules/user'

const userStore = useUserStore()
</script>
```

### 组合式函数封装

将跨 store 的业务逻辑封装为 composable：

```typescript
// composables/useAuth.ts
import useUserStore from '@/store/modules/user'

export function useAuth() {
  const userStore = useUserStore()

  /**
   * 检查是否拥有指定权限
   * @param ids - 权限 ID 数组（任一匹配即可）
   */
  function hasPermission(ids: string[]): boolean {
    return userStore.VA(ids)
  }

  /**
   * 检查是否为管理员
   */
  function isAdmin(): boolean {
    return userStore.username === 'admin' || userStore.username === 'root'
  }

  /**
   * 获取当前用户显示名称
   */
  function displayName(): string {
    return userStore.userInfo?.nickname || userStore.username
  }

  return { hasPermission, isAdmin, displayName }
}
```

```typescript
// composables/useMenu.ts
import useMenuStore from '@/store/modules/menu'
import useSettingsStore from '@/store/modules/settings'

export function useMenu() {
  const menuStore = useMenuStore()
  const settingsStore = useSettingsStore()

  const isSingleMode = computed(
    () => settingsStore.settings.menu.menuMode === 'single',
  )

  function switchTo(index: number | string) {
    menuStore.setActived(index)
  }

  return {
    allMenus: computed(() => menuStore.allMenus),
    sidebarMenus: computed(() => menuStore.sidebarMenus),
    activeIndex: computed(() => menuStore.actived),
    isSingleMode,
    switchTo,
  }
}
```

### 多 Tab 同步

通过 `StorageEvent` 实现多标签页间的登录状态同步：

```typescript
// utils/cross-tab-sync.ts
export function setupCrossTabSync() {
  window.addEventListener('storage', (event) => {
    if (event.key === 'token') {
      // token 变化时，同步到 store
      const userStore = useUserStore()
      userStore.syncTokenFromStorage()

      // token 被清除（其他标签页登出）
      if (!event.newValue) {
        userStore.clearLocalLoginState()
        router.push({ name: 'login' })
      }
    }

    if (event.key === 'heartbeat:logout') {
      // 心跳检测到离线，强制登出
      const userStore = useUserStore()
      userStore.logout(undefined, { callServer: false, broadcast: false })
    }
  })
}
```

```typescript
// utils/heartbeat.ts
export function broadcastHeartbeatLogout() {
  localStorage.setItem('heartbeat:logout', Date.now().toString())
  localStorage.removeItem('heartbeat:logout')
}

export function broadcastHeartbeatTokenUpdated() {
  localStorage.setItem('heartbeat:token-updated', Date.now().toString())
  localStorage.removeItem('heartbeat:token-updated')
}
```

## 实际场景

### 场景一：登录流程

```typescript
// views/login.vue
const userStore = useUserStore()
const router = useRouter()

async function handleLogin() {
  try {
    await userStore.login({
      username: form.username,
      password: form.password,
      ua: navigator.userAgent,
    })
    // 登录成功后跳转到 redirect 参数或默认首页
    const redirect = router.currentRoute.value.query.redirect as string
    router.push(redirect || { name: 'home' })
  } catch (error) {
    ElMessage.error('登录失败，请检查用户名和密码')
  }
}
```

### 场景二：页面权限拦截

```vue
<script setup lang="ts">
import { useAuth } from '@/composables/useAuth'

const { hasPermission, isAdmin } = useAuth()

// 无权限时显示提示
const canExport = computed(() => hasPermission(['sys:user:export']) || isAdmin())
</script>

<template>
  <div>
    <el-button :disabled="!canExport" @click="handleExport">
      导出数据
    </el-button>
    <el-alert
      v-if="!canExport"
      title="您没有导出权限，请联系管理员"
      type="warning"
      show-icon
    />
  </div>
</template>
```

### 场景三：主题切换

```typescript
// components/ThemeSwitch.vue
const settingsStore = useSettingsStore()

function toggleTheme() {
  const current = settingsStore.settings.app.colorScheme
  settingsStore.setColorScheme(current === 'dark' ? 'light' : 'dark')
}
```

## 工作原理

### Store 初始化流程

```text
应用启动
  |
  +-- createPinia()
  |
  +-- app.use(pinia)
  |
  +-- 首次组件渲染
  |     |
  |     +-- useXxxStore() 首次调用触发 defineStore 的 setup 函数
  |     +-- ref/computed/watch 按需初始化
  |
  +-- beforeEach 守卫
        |
        +-- useUserStore().getPermissions()    // 拉取权限
        +-- usePermissionStore().setRoutes()    // 生成路由
        +-- useRouteStore().generateRoutes()    // 注册路由
```

### 状态持久化策略

| Store | 持久化方式 | 存储位置 | 刷新保留 |
|-------|-----------|---------|---------|
| user (token) | 手动 localStorage | `localStorage.token` | 是 |
| user (refreshToken) | 手动 localStorage | `localStorage.refreshToken` | 是 |
| user (username) | 手动 localStorage | `localStorage.username` | 是 |
| user (menus) | 手动 localStorage | `localStorage.menus` | 是 |
| settings | 内存 + watch | 内存（含默认值） | 否（重置为默认） |
| permission | 内存 | 无 | 否（重新生成） |
| route | 内存 | 无 | 否（重新生成） |
| menu | 内存 | 无 | 否（重新生成） |
| keepAlive | 内存 | 无 | 否（重新生成） |

::: tip 手动持久化
`user` store 的持久化是手动管理的（在 `login`/`logout` 中显式操作 `localStorage`），而非使用 `pinia-plugin-persistedstate`。这样做可以精确控制哪些字段需要持久化以及存储位置。
:::

### Store 间依赖关系

```text
user store
  +-- depends on: route store, menu store (logout 时重置)
  +-- used by: permission store, menu store, route guard

permission store
  +-- depends on: user store (获取用户名判断是否为 admin)
  +-- used by: route guard (注册动态路由)

menu store
  +-- depends on: settings store, route store, user store
  +-- used by: layout 组件（侧边栏、面包屑）

settings store
  +-- depends on: 无（独立模块）
  +-- used by: route guard, layout 组件, 主题系统

keepAlive store
  +-- depends on: 无（独立模块）
  +-- used by: route afterEach, layout 组件
```

## 常见问题

### Token 刷新后其他标签页未同步

**原因**：未监听 `storage` 事件。

**解决方案**：在应用初始化时调用 `setupCrossTabSync()`，监听 `token` 键的 `StorageEvent`。

### 权限校验返回不正确

**原因**：`menus` 字段为空或格式错误。

**排查步骤**：

1. 检查 `localStorage.getItem('menus')` 的值。
2. 确认后端 `getMenus` 接口返回的是逗号分隔的权限 ID 字符串。
3. 确认 `routeMenu.ts` 中配置的 `id` 与后端一致。

```typescript
// 调试权限
const userStore = useUserStore()
console.log('menus:', userStore.menus)
console.log('VA result:', userStore.VA(['sys:user:add']))
```

### Store 跨组件引用报错

**原因**：在 `defineStore` 的 setup 函数中循环引用其他 store。

**解决方案**：在函数体内部（而非顶层）引用其他 store：

```typescript
// 错误：顶层引用可能导致循环依赖
const useAStore = defineStore('a', () => {
  const bStore = useBStore() // 如果 B 也引用 A，会循环
})

// 正确：在 action 内部引用
const useAStore = defineStore('a', () => {
  function doSomething() {
    const bStore = useBStore() // 延迟引用，避免循环
    // ...
  }
})
```

### KeepAlive 列表无限增长

**原因**：`add` 方法未做去重检查（实际代码已做），或 afterEach 中缓存逻辑未正确移除。

**排查步骤**：

1. 在控制台输出 `keepAliveStore.list` 观察变化。
2. 确认页面组件的 `name` 属性设置正确。
3. 检查 `meta.cache` 配置是否合理。

## 参考链接

- [Pinia 官方文档](https://pinia.vuejs.org/zh/)
- [Vue 3 组合式 API](https://cn.vuejs.org/guide/extras/composition-api-faq.html)
- [前端路由与权限](./routing.md)
- [主题与设计系统](./theme.md)
- [前端开发总览](../frontend-development.md)
