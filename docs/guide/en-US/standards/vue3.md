# Vue3 Frontend Development Standards

This document defines the Vue3 development standards for the EgoAdmin frontend project (`web/`), covering TypeScript, component development, state management, routing and permissions, styling and theming, the API layer, internationalization, linting, and build configuration.

## Overview

The EgoAdmin frontend is built on Vite + Vue 3 Composition API + TypeScript, using Element Plus as the UI component library, Pinia for state management, Vue Router for routing and permissions, and vue-i18n for multilingual support. The project uses file-system routing (vite-plugin-pages), auto-imports (unplugin-auto-import / unplugin-vue-components), and an SCSS design token system.

All business APIs are called through gRPC HTTP-compatible paths. API modules and types are auto-generated from backend proto definitions (`api-manifest.ts`). Route permissions are dynamically injected from the backend menu/permission API via `beforeEach` guards that handle login checks and dynamic route generation.

This standard defines the complete frontend development guidelines from project structure and TypeScript configuration to coding style and anti-pattern avoidance, ensuring team code consistency and maintainability.

## Core Usage

### TypeScript Configuration

The project uses strict TypeScript with path aliases and global types:

```ts
// tsconfig.json key settings
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "paths": {
      "@/*": ["./src/*"],     // src alias
      "#/*": ["./src/types/*"] // types alias
    },
    "types": [
      "element-plus/global",             // Element Plus global types
      "vite-plugin-pages/client",        // File routing types
      "vite-plugin-vue-meta-layouts/client" // Layout meta types
    ]
  }
}
```

Use `@/` to reference modules under `src/` and `#/` to reference type declarations under `src/types/`.

### Component Development

Use `<script setup lang="ts">` syntax. Component tag order: route comment -> script -> template -> style:

```vue
<script setup lang="ts">
// 1. Props definition
interface Props {
  userId: string
  showAvatar?: boolean
}
const props = withDefaults(defineProps<Props>(), {
  showAvatar: true,
})

// 2. Emits definition
const emit = defineEmits<{
  (e: 'update', id: string): void
  (e: 'delete', id: string): void
}>()

// 3. Store
const userStore = useUserStore()

// 4. Composable
const { hasPermission } = useAuth()

// 5. State & computed
const loading = ref(false)
const displayName = computed(() => userStore.userInfo?.name ?? '')

// 6. Methods
async function handleUpdate() {
  loading.value = true
  try {
    emit('update', props.userId)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div v-if="props.showAvatar" class="user-card">
    <span>{{ displayName }}</span>
    <el-button @click="handleUpdate">Update</el-button>
  </div>
</template>

<style lang="scss" scoped>
.user-card {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
```

::: tip Component File Organization
- Use PascalCase for folders with `index.vue` as the root component
- Shared components go in `src/components/`; business components stay co-located with their pages
- Composables go in `src/composables/`, e.g. `useAuth()`, `useMenu()`
:::

### State Management (Pinia)

Stores use the Composition API style (`setup` function) with modular organization:

```ts
// store/modules/user.ts
import apiUser from '@/api/modules/user'

const useUserStore = defineStore('user', () => {
  // State
  const token = ref(localStorage.getItem('token') ?? '')
  const userInfo = ref<any>({})
  const permissions = ref<string[]>([])

  // Derived state
  const isLogin = computed(() => !!token.value)

  // Actions - sync
  function setToken(val: string) {
    token.value = val
    localStorage.setItem('token', val)
  }

  // Actions - async (loading/error pattern)
  async function bootstrapSession() {
    try {
      const { data } = await apiUser.getCenterInfo()
      userInfo.value = data
      permissions.value = data.permissions ?? []
    } catch (error) {
      console.error('Session bootstrap failed', error)
      clearLocalLoginState()
    }
  }

  function clearLocalLoginState() {
    localStorage.removeItem('token')
    token.value = ''
    userInfo.value = {}
    permissions.value = []
  }

  return { token, userInfo, permissions, isLogin, setToken, bootstrapSession, clearLocalLoginState }
})

export default useUserStore
```

Store modules: `settings`, `user`, `permission`, `route`, `menu`, `keepAlive`.

::: warning Important
Do not directly modify another store's state from a component. Expose operations through store methods. Use `computed` for derived data and avoid redundant state.
:::

### Routing & Permissions

Route guards are defined in `router/index.ts`. The core flow is login check -> dynamic route generation -> permission validation:

```ts
// router/index.ts core logic
router.beforeEach(async (to, from, next) => {
  const userStore = useUserStore()
  const routeStore = useRouteStore()
  const settingsStore = useSettingsStore()

  // Progress bar
  settingsStore.settings.app.enableProgress && (isLoading.value = true)

  if (userStore.isLogin) {
    if (routeStore.isGenerate) {
      // Dynamic routes already generated, navigate normally
      next()
    } else {
      // First login, fetch menus from backend and generate routes
      await routeStore.generateRoutes()
      next({ ...to, replace: true })
    }
  } else {
    // Not logged in, redirect to login (except whitelisted routes)
    if (to.meta.whiteList) {
      next()
    } else {
      next({ path: '/login', query: { redirect: to.fullPath } })
    }
  }
})
```

Route meta for keepAlive cache control:

```ts
// Page route meta example
defineOptions({
  meta: {
    title: 'route.userManagement', // i18n key
    cache: true,                    // enable keepAlive
  },
})
```

### API Layer

API modules are organized by domain, all using `api.post` with gRPC HTTP-compatible paths:

```ts
// api/modules/user.ts
import api from '../index'

export default {
  getLoginCrypto: (data: { username: string; ua: string; action?: string }) =>
    api.post('/user.v1.UserService/GetLoginCrypto', data),

  login: (data: {
    username?: string
    passwordCipher?: string
    keyId?: string
    challengeId?: string
  }) => api.post('/user.v1.UserService/Login', data),

  getCenterInfo: () =>
    api.post('/user.v1.CenterService/GetCenterInfo'),
}
```

```ts
// api/index.ts - Axios instance & interceptors
import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_APP_API_BASEURL,
  timeout: 30000,
})

// Request interceptor: inject Authorization
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Response interceptor: unified error handling
api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    if (error.response?.status === 401) {
      // Token expired, clear state and redirect to login
      useUserStore().clearLocalLoginState()
      router.push('/login')
    }
    return Promise.reject(error)
  },
)

export default api
```

::: tip API Rules
- Use `api.post` instead of RESTful GET/PUT/DELETE
- Body fields use protobuf JSON field names
- Backend `uint64` IDs are treated as `string` in the frontend
- `api-manifest.ts` is auto-generated from backend proto; do not edit manually
:::

### Internationalization (i18n)

Uses vue-i18n with translation files organized by language under `src/i18n/package/`:

```ts
// i18n/index.ts
import { createI18n } from 'vue-i18n'
import zhCN from './package/zh-CN'
import enUS from './package/en-US'

export type LocaleKey = 'zh-CN' | 'en'

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: initialLocale, // read from localStorage or navigator.language
  fallbackLocale: 'zh-CN',
  messages: { 'zh-CN': zhCN, en: enUS },
})
```

Route titles use i18n keys, dynamically set in `App.vue` via `useTitle` + `t()`. The backend propagates language preference through the `Accept-Language` header.

### Styling & Theming

Uses SCSS variables + CSS custom properties for theming:

```scss
// styles/resources/variables.scss - design tokens
$g-sidebar-width: 220px;
$g-header-height: 50px;

// styles/globals.scss - global style entry
// SCSS files in resources/ are auto-injected into all components via vite.config.ts
```

```vue
<!-- Component styles -->
<style lang="scss" scoped>
// BEM naming convention
.user-card {
  &__avatar {
    width: 40px;
    height: 40px;
    border-radius: 50%;
  }
  &__name {
    font-size: 14px;
    color: var(--el-text-color-primary);
  }
}
</style>
```

::: warning Styling Rules
- Use `scoped` + SCSS for component styles
- Follow BEM naming: `.block__element--modifier`
- Global variables are auto-injected through `styles/resources/`; no manual import needed
- Element Plus theming configured via ConfigProvider for language, size, and buttons
:::

## Configuration Examples

### Vite Build Configuration

| Config | Value | Description |
|--------|-------|-------------|
| `base` | `'./'` | Relative path deployment |
| `server.port` | `9000` | Dev server port |
| `server.proxy` | `/proxy` -> `VITE_APP_API_BASEURL` | API proxy |
| `build.outDir` | `dist` / `dist-${mode}` | Output directory |
| `css.preprocessorOptions.scss.additionalData` | Auto-inject from resources/ | Global SCSS resources |

### Vite Plugins

| Plugin | Purpose |
|--------|---------|
| `@vitejs/plugin-vue` | Vue 3 SFC support |
| `vite-plugin-pages` | File-system routing |
| `vite-plugin-vue-meta-layouts` | Layout system |
| `unplugin-auto-import` | Auto-import APIs (ref, computed, etc.) |
| `unplugin-vue-components` | Auto-register components |
| `vite-plugin-svg-icons-ng` | SVG icons |
| `vite-plugin-mock` | Mock data |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `VITE_APP_API_BASEURL` | Backend API base URL |
| `VITE_OPEN_PROXY` | Enable proxy (`true`/`false`) |
| `VITE_BUILD_SOURCEMAP` | Generate sourcemap |

### Lint Configuration

| Tool | Config File | Key Rules |
|------|------------|-----------|
| ESLint | `.eslintrc` (oxlint) | `no-console: off`, `no-var: error`, `prefer-const: error`, `curly: always` |
| StyleLint | `.stylelintrc` | SCSS extensions + standard rules |
| Formatter | `vite.config.ts` fmt field | No semicolons, single quotes |

### Common Scripts

| Command | Description |
|---------|-------------|
| `pnpm dev` | Start dev server |
| `pnpm build` | Type check + build + generate permission contract |
| `pnpm lint` | Run lint checks |
| `pnpm format` | Format code |
| `pnpm type-check` | TypeScript type check only |
| `pnpm new` | Plop code generation |

## Real-World Examples

### User Login Flow

The complete login flow involves API calls, encryption, store state updates, and route navigation:

```vue
<script setup lang="ts">
import apiUser from '@/api/modules/user'
import { encryptPasswordPayload } from '@/utils/login-crypto'

const router = useRouter()
const userStore = useUserStore()

const loginForm = reactive({
  username: '',
  password: '',
})
const loading = ref(false)

async function handleLogin() {
  loading.value = true
  try {
    // 1. Fetch encryption parameters
    const cryptoRes = await apiUser.getLoginCrypto({
      username: loginForm.username,
      ua: navigator.userAgent,
    })

    // 2. Encrypt password
    const payload = await encryptPasswordPayload(
      cryptoRes.data,
      loginForm.password,
    )

    // 3. Call login API
    const res = await apiUser.login({
      username: loginForm.username,
      ...payload,
    })

    // 4. Save token and initialize session
    userStore.setToken(res.data.token)
    await userStore.bootstrapSession()

    // 5. Redirect to target page or home
    const redirect = router.currentRoute.value.query.redirect as string
    router.push(redirect || '/')
  } catch (error) {
    console.error('Login failed', error)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <el-form :model="loginForm" @submit.prevent="handleLogin">
    <el-form-item>
      <el-input v-model="loginForm.username" placeholder="Username" />
    </el-form-item>
    <el-form-item>
      <el-input v-model="loginForm.password" type="password" placeholder="Password" />
    </el-form-item>
    <el-button type="primary" :loading="loading" @click="handleLogin">
      Login
    </el-button>
  </el-form>
</template>
```

Flow explanation: First fetch server-side encryption parameters (public key + keyId + challengeId), encrypt the password with the public key, then submit. After successful login, save the token, call `bootstrapSession` to load user info and permissions, and finally redirect based on the `redirect` query parameter.

### List Page with Skeleton & Empty States

A typical list page includes loading skeleton, empty state, and data display:

```vue
<script setup lang="ts">
import apiUser from '@/api/modules/user'

const loading = ref(false)
const tableData = ref<any[]>([])
const pagination = reactive({ page: 1, limit: 20, total: 0 })

async function fetchData() {
  loading.value = true
  try {
    const { data } = await apiUser.getUserList({
      page: pagination.page,
      limit: pagination.limit,
    })
    tableData.value = data.list ?? []
    pagination.total = data.total ?? 0
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<template>
  <div class="user-list">
    <!-- Skeleton loading -->
    <el-table v-if="loading && !tableData.length" :data="Array(5).fill({})">
      <el-table-column v-for="i in 4" :key="i">
        <template #default>
          <el-skeleton :rows="1" animated />
        </template>
      </el-table-column>
    </el-table>

    <!-- Empty state -->
    <el-empty v-else-if="!tableData.length" description="No data" />

    <!-- Data table -->
    <el-table v-else :data="tableData">
      <el-table-column prop="username" label="Username" />
      <el-table-column prop="name" label="Name" />
      <el-table-column prop="phone" label="Phone" />
      <el-table-column label="Actions">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleEdit(row)">Edit</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="pagination.page"
      :total="pagination.total"
      :page-size="pagination.limit"
      @current-change="fetchData"
    />
  </div>
</template>
```

### Composable for Permission Checks

```ts
// composables/useAuth.ts
export function useAuth() {
  const userStore = useUserStore()

  function hasPermission(code: string): boolean {
    return userStore.permissions.includes(code)
  }

  function hasAnyPermission(codes: string[]): boolean {
    return codes.some((code) => userStore.permissions.includes(code))
  }

  return { hasPermission, hasAnyPermission }
}
```

```vue
<!-- Usage in template -->
<script setup lang="ts">
const { hasPermission } = useAuth()
</script>

<template>
  <el-button v-if="hasPermission('user:create')" type="primary">
    New User
  </el-button>
</template>
```

## Common Issues

**Q: Is it acceptable to use the `any` type?**

No. `any` bypasses TypeScript type checking and makes runtime errors undetectable at compile time. Use concrete types, `unknown` + type guards, or generic constraints instead. For third-party libraries lacking types, use `declare module` to add type declarations.

**Q: Why shouldn't route paths be hardcoded?**

Hardcoded route paths (e.g., `router.push('/system/user')`) are easily missed during refactoring. Use named routes instead: `router.push({ name: 'SystemUser' })`. Named routes allow the compiler to assist with path change detection.

**Q: What if API calls lack error handling?**

All API calls must have `try/catch` or `.catch()` handling. Use the `loading` + `error` state pattern:

```ts
const loading = ref(false)
const error = ref<string | null>(null)

async function fetchData() {
  loading.value = true
  error.value = null
  try {
    const res = await apiUser.getUserList(params)
    // Handle success
  } catch (e) {
    error.value = (e as Error).message
    ElMessage.error('Request failed')
  } finally {
    loading.value = false
  }
}
```

**Q: How to properly use keepAlive caching?**

Set `cache: true` in route meta. The `keepAlive` store automatically manages the cached component list. The `afterEach` guard handles cleanup when pages are exited. Do not manually manipulate the `keep-alive` include list inside components.

**Q: Can I directly manipulate the DOM in components?**

No. Vue's reactivity system handles DOM updates. Direct DOM manipulation causes state inconsistency. If you need to operate on third-party DOM (e.g., chart libraries), use `ref` + `onMounted`/`watch` within controlled lifecycle hooks.

**Q: Why can't I use the `@/` alias in library code?**

The `@/` alias is resolved by Vite at build time and only works within the `web/` project. If code is extracted as a standalone library, the alias cannot be resolved. Library code should use relative paths.

## Reference Links

- `web/tsconfig.json` - TypeScript configuration
- `web/vite.config.ts` - Vite build configuration
- `web/src/main.ts` - Application entry point
- `web/src/App.vue` - Root component (language switching, hotkeys, debug tools)
- `web/src/router/index.ts` - Route guards and dynamic routing
- `web/src/store/modules/` - Pinia store modules
- `web/src/api/modules/` - API domain modules
- `web/src/api/index.ts` - Axios instance and interceptors
- `web/src/api/api-manifest.ts` - Auto-generated API manifest
- `web/src/i18n/index.ts` - Internationalization configuration
- `web/src/i18n/package/` - Translation files
- `web/src/composables/` - Composable functions
- `web/src/styles/resources/` - Global SCSS resources
- `web/src/layouts/` - Layout components
- `web/package.json` - Dependencies and scripts
