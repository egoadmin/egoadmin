# State Management

EgoAdmin uses Pinia for global state management, split into independent modules by business domain, combined with composable functions for reusable business logic.

## Overview

The project uses a **single Pinia instance + multi-module** architecture. All stores are located under `store/modules/`:

```text
web/src/store/modules/
├── user.ts          # User state: login, token, permissions, session
├── permission.ts    # Permission routes: dynamic route generation and filtering
├── route.ts         # Route state: registered routes and system routes
├── menu.ts          # Menu state: nav tree, active item, breadcrumbs
├── keepAlive.ts     # Cache state: page-level component cache list
└── settings.ts      # Settings state: theme, language, layout preferences
```

Each store uses the Composition API style (`defineStore` with `setup` function), consistent with Vue 3 `<script setup>` conventions.

## Core Usage

### User Store

The user store is the core of the authentication system, managing login state, tokens, user info, and permission checking:

```typescript
// store/modules/user.ts
import useRouteStore from './route'
import useMenuStore from './menu'
import router, { resetRouter } from '@/router'
import apiUser from '@/api/modules/user'

const useUserStore = defineStore('user', () => {
  // ---------- State ----------
  const token = ref(localStorage.getItem('token') ?? '')
  const refreshToken = ref(localStorage.getItem('refreshToken') ?? '')
  const userInfo: any = ref({})
  const username: any = ref(localStorage.getItem('username') ?? '')
  const permissions = ref<string[]>([])
  const menus = ref(localStorage.getItem('menus') ?? '')

  const isLogin = computed(() => !!token.value)

  // ---------- Login ----------
  async function login(data: {
    username?: string
    password?: string
    ua?: string
    token?: string
  }) {
    let loginData: any = data
    if (!data.token) {
      // Encrypt password using LoginCrypto
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

  // ---------- Logout ----------
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

  // ---------- Permission Check ----------
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

The `VA` method handles button-level permission checking. It accepts an array of permission IDs and returns whether the current user has any of them.

### Permission Store

Responsible for filtering and generating dynamic routes based on user permissions:

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
    // Match routes against permissions, return allowed children
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
    // Recursively extract permission-matched paths
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
    // Ensure parent paths are included
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

::: warning admin/root Bypass Filtering
`admin` and `root` users get all routes without permission filtering. Ensure these accounts are secure in production.
:::

### Menu Store

Manages the navigation menu tree, main navigation active state, and breadcrumbs:

```typescript
// store/modules/menu.ts
const useMenuStore = defineStore('menu', () => {
  const settingsStore = useSettingsStore()
  const routeStore = useRouteStore()

  const menus = ref<Menu.recordMainRaw[]>([{ meta: {}, children: [] }])
  const actived = ref(0)

  // Full navigation data
  const allMenus = computed(() => {
    let returnMenus: Menu.recordMainRaw[] = [{ meta: {}, children: [] }]
    if (settingsStore.settings.app.routeBaseOn !== 'filesystem') {
      if (settingsStore.settings.menu.menuMode === 'single') {
        // Single nav mode: merge all top-level route children
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

  // Sidebar navigation data
  const sidebarMenus = computed(() => {
    return allMenus.value.length > 0
      ? allMenus.value[actived.value].children
      : []
  })

  // Switch main navigation
  function setActived(data: number | string) {
    if (typeof data === 'number') {
      actived.value = data
    } else {
      // Locate main nav index by route path
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

### KeepAlive Store

Manages the page-level component cache list, used with Vue's `<KeepAlive>`:

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

Used in the layout component:

```vue
<template>
  <router-view v-slot="{ Component }">
    <keep-alive :include="keepAliveStore.list">
      <component :is="Component" />
    </keep-alive>
  </router-view>
</template>
```

### Settings Store

Manages global settings including theme, layout, and user preferences:

```typescript
// store/modules/settings.ts
import { defaultsDeep } from 'lodash-es'
import settingsCustom from '@/settings'
import settingsDefault from '@/settings.default'

const useSettingsStore = defineStore('settings', () => {
  const mergeSettings = defaultsDeep(settingsCustom, settingsDefault)
  const settings = ref(mergeSettings)

  // Theme toggle: watch colorScheme changes, manipulate root element class
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

  // Mobile adaptation
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

  // Sidebar toggle
  function toggleSidebarCollapse() {
    settings.value.menu.subMenuCollapse = !settings.value.menu.subMenuCollapse
  }

  // Page title
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

## Configuration Examples

### Permission Checking in Templates

```vue
<template>
  <div>
    <!-- Button-level permission: only visible to authorized users -->
    <el-button
      v-if="userStore.VA(['sys:user:add'])"
      type="primary"
      @click="handleAdd"
    >
      Add User
    </el-button>

    <el-button
      v-if="userStore.VA(['sys:user:edit'])"
      @click="handleEdit"
    >
      Edit
    </el-button>

    <el-button
      v-if="userStore.VA(['sys:user:delete'])"
      type="danger"
      @click="handleDelete"
    >
      Delete
    </el-button>
  </div>
</template>

<script setup lang="ts">
import useUserStore from '@/store/modules/user'

const userStore = useUserStore()
</script>
```

### Composable Encapsulation

Encapsulate cross-store business logic into composables:

```typescript
// composables/useAuth.ts
import useUserStore from '@/store/modules/user'

export function useAuth() {
  const userStore = useUserStore()

  /**
   * Check if user has any of the specified permissions
   * @param ids - Permission ID array (any match suffices)
   */
  function hasPermission(ids: string[]): boolean {
    return userStore.VA(ids)
  }

  /**
   * Check if user is an admin
   */
  function isAdmin(): boolean {
    return userStore.username === 'admin' || userStore.username === 'root'
  }

  /**
   * Get current user display name
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

### Multi-Tab Synchronization

Implement cross-tab login state synchronization using `StorageEvent`:

```typescript
// utils/cross-tab-sync.ts
export function setupCrossTabSync() {
  window.addEventListener('storage', (event) => {
    if (event.key === 'token') {
      // Sync token to store when it changes
      const userStore = useUserStore()
      userStore.syncTokenFromStorage()

      // Token cleared (another tab logged out)
      if (!event.newValue) {
        userStore.clearLocalLoginState()
        router.push({ name: 'login' })
      }
    }

    if (event.key === 'heartbeat:logout') {
      // Heartbeat detected offline, force logout
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

## Real-World Examples

### Example 1: Login Flow

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
    // After login, redirect to the query param or default home
    const redirect = router.currentRoute.value.query.redirect as string
    router.push(redirect || { name: 'home' })
  } catch (error) {
    ElMessage.error('Login failed, please check username and password')
  }
}
```

### Example 2: Page Permission Guard

```vue
<script setup lang="ts">
import { useAuth } from '@/composables/useAuth'

const { hasPermission, isAdmin } = useAuth()

// Show hint when no permission
const canExport = computed(() => hasPermission(['sys:user:export']) || isAdmin())
</script>

<template>
  <div>
    <el-button :disabled="!canExport" @click="handleExport">
      Export Data
    </el-button>
    <el-alert
      v-if="!canExport"
      title="You do not have export permission. Please contact an administrator."
      type="warning"
      show-icon
    />
  </div>
</template>
```

### Example 3: Theme Toggle

```typescript
// components/ThemeSwitch.vue
const settingsStore = useSettingsStore()

function toggleTheme() {
  const current = settingsStore.settings.app.colorScheme
  settingsStore.setColorScheme(current === 'dark' ? 'light' : 'dark')
}
```

## How It Works

### Store Initialization Flow

```text
App startup
  |
  +-- createPinia()
  |
  +-- app.use(pinia)
  |
  +-- First component render
  |     |
  |     +-- useXxxStore() first call triggers defineStore's setup function
  |     +-- ref/computed/watch initialized on demand
  |
  +-- beforeEach guard
        |
        +-- useUserStore().getPermissions()    // Fetch permissions
        +-- usePermissionStore().setRoutes()    // Generate routes
        +-- useRouteStore().generateRoutes()    // Register routes
```

### State Persistence Strategy

| Store | Persistence | Storage | Survives Refresh |
|-------|------------|---------|-----------------|
| user (token) | Manual localStorage | `localStorage.token` | Yes |
| user (refreshToken) | Manual localStorage | `localStorage.refreshToken` | Yes |
| user (username) | Manual localStorage | `localStorage.username` | Yes |
| user (menus) | Manual localStorage | `localStorage.menus` | Yes |
| settings | Memory + watch | Memory (with defaults) | No (reset to defaults) |
| permission | Memory | None | No (regenerated) |
| route | Memory | None | No (regenerated) |
| menu | Memory | None | No (regenerated) |
| keepAlive | Memory | None | No (regenerated) |

::: tip Manual Persistence
The `user` store uses manual persistence (explicit `localStorage` operations in `login`/`logout`) rather than `pinia-plugin-persistedstate`. This gives precise control over which fields are persisted and their storage locations.
:::

### Store Dependency Graph

```text
user store
  +-- depends on: route store, menu store (reset on logout)
  +-- used by: permission store, menu store, route guard

permission store
  +-- depends on: user store (username check for admin)
  +-- used by: route guard (register dynamic routes)

menu store
  +-- depends on: settings store, route store, user store
  +-- used by: layout component (sidebar, breadcrumbs)

settings store
  +-- depends on: none (standalone module)
  +-- used by: route guard, layout component, theme system

keepAlive store
  +-- depends on: none (standalone module)
  +-- used by: route afterEach, layout component
```

## Common Issues

### Token Not Synced Across Tabs After Refresh

**Cause**: `storage` event not being listened to.

**Solution**: Call `setupCrossTabSync()` during app initialization to listen for `StorageEvent` on the `token` key.

### Permission Check Returns Incorrect Results

**Cause**: `menus` field is empty or has wrong format.

**Troubleshooting**:

1. Check the value of `localStorage.getItem('menus')`.
2. Confirm the backend `getMenus` API returns a comma-separated permission ID string.
3. Confirm `routeMenu.ts` IDs match the backend.

```typescript
// Debug permissions
const userStore = useUserStore()
console.log('menus:', userStore.menus)
console.log('VA result:', userStore.VA(['sys:user:add']))
```

### Store Cross-Reference Errors

**Cause**: Circular reference between stores when referencing at the top level of `defineStore` setup.

**Solution**: Reference other stores inside function bodies (not at the top level):

```typescript
// Wrong: top-level reference may cause circular dependency
const useAStore = defineStore('a', () => {
  const bStore = useBStore() // If B also references A, it cycles
})

// Correct: reference inside action body
const useAStore = defineStore('a', () => {
  function doSomething() {
    const bStore = useBStore() // Deferred reference avoids cycles
    // ...
  }
})
```

### KeepAlive List Growing Indefinitely

**Cause**: The `add` method lacks deduplication (the actual code already handles this), or the afterEach cache logic fails to remove entries properly.

**Troubleshooting**:

1. Output `keepAliveStore.list` in the console to observe changes.
2. Confirm page component `name` properties are set correctly.
3. Check that `meta.cache` configuration is reasonable.

## Reference Links

- [Pinia Official Documentation](https://pinia.vuejs.org/)
- [Vue 3 Composition API FAQ](https://vuejs.org/guide/extras/composition-api-faq.html)
- [Frontend Routing and Permissions](./routing.md)
- [Theme and Design System](./theme.md)
- [Frontend Development Overview](../frontend-development.md)
