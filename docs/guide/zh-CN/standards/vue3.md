# Vue3 前端开发规范

本文档为 EgoAdmin 前端项目（`web/`）的 Vue3 开发规范，涵盖 TypeScript、组件开发、状态管理、路由权限、样式主题、API 层、国际化、Lint 规范与构建配置。

## 概述

EgoAdmin 前端基于 Vite + Vue 3 Composition API + TypeScript 构建，使用 Element Plus 作为 UI 组件库，Pinia 管理状态，Vue Router 处理路由与权限，vue-i18n 实现多语言。项目采用文件系统路由（vite-plugin-pages）、自动导入（unplugin-auto-import / unplugin-vue-components）和 SCSS 设计令牌体系。

所有业务 API 通过 gRPC HTTP 兼容路径调用，API 模块及类型从后端 proto 自动生成（`api-manifest.ts`）。路由权限由后端菜单/权限接口动态注入，通过 `beforeEach` 守卫完成登录检查与动态路由生成。

本规范定义了从项目结构、TypeScript 配置到编码风格、反模式规避的完整前端开发标准，确保团队代码一致性与可维护性。

## 核心用法

### TypeScript 配置

项目 TypeScript 配置严格模式开启，路径别名与全局类型如下：

```ts
// tsconfig.json 关键配置
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "paths": {
      "@/*": ["./src/*"],     // src 别名
      "#/*": ["./src/types/*"] // 类型别名
    },
    "types": [
      "element-plus/global",             // Element Plus 全局类型
      "vite-plugin-pages/client",        // 文件路由类型
      "vite-plugin-vue-meta-layouts/client" // 布局元信息类型
    ]
  }
}
```

使用 `@/` 引用 `src/` 下的模块，`#/` 引用 `src/types/` 下的类型声明。

### 组件开发

使用 `<script setup lang="ts">` 语法，组件标签顺序为 route comment -> script -> template -> style：

```vue
<script setup lang="ts">
// 1. Props 定义
interface Props {
  userId: string
  showAvatar?: boolean
}
const props = withDefaults(defineProps<Props>(), {
  showAvatar: true,
})

// 2. Emits 定义
const emit = defineEmits<{
  (e: 'update', id: string): void
  (e: 'delete', id: string): void
}>()

// 3. Store
const userStore = useUserStore()

// 4. Composable
const { hasPermission } = useAuth()

// 5. 状态与计算属性
const loading = ref(false)
const displayName = computed(() => userStore.userInfo?.name ?? '')

// 6. 方法
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
    <el-button @click="handleUpdate">更新</el-button>
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

::: tip 组件文件组织
- 文件夹使用 PascalCase，根组件为 `index.vue`
- 公共组件放在 `src/components/`，业务组件就近放在页面目录
- Composables 放在 `src/composables/`，如 `useAuth()`、`useMenu()`
:::

### 状态管理 (Pinia)

Store 使用 Composition API 风格（`setup` 函数），模块化组织：

```ts
// store/modules/user.ts
import apiUser from '@/api/modules/user'

const useUserStore = defineStore('user', () => {
  // 状态
  const token = ref(localStorage.getItem('token') ?? '')
  const userInfo = ref<any>({})
  const permissions = ref<string[]>([])

  // 派生状态
  const isLogin = computed(() => !!token.value)

  // 操作 - 同步
  function setToken(val: string) {
    token.value = val
    localStorage.setItem('token', val)
  }

  // 操作 - 异步 (loading/error 模式)
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

Store 模块列表：`settings`、`user`、`permission`、`route`、`menu`、`keepAlive`。

::: warning 注意
不要在组件中直接修改其他 store 的状态，应通过 store 方法暴露操作。派生数据使用 `computed`，避免冗余状态。
:::

### 路由与权限

路由守卫在 `router/index.ts` 中定义，核心流程为登录检查 -> 动态路由生成 -> 权限校验：

```ts
// router/index.ts 核心逻辑
router.beforeEach(async (to, from, next) => {
  const userStore = useUserStore()
  const routeStore = useRouteStore()
  const settingsStore = useSettingsStore()

  // 进度条
  settingsStore.settings.app.enableProgress && (isLoading.value = true)

  if (userStore.isLogin) {
    if (routeStore.isGenerate) {
      // 已生成动态路由，正常导航
      next()
    } else {
      // 首次登录，从后端获取菜单生成路由
      await routeStore.generateRoutes()
      next({ ...to, replace: true })
    }
  } else {
    // 未登录，跳转登录页（白名单路由除外）
    if (to.meta.whiteList) {
      next()
    } else {
      next({ path: '/login', query: { redirect: to.fullPath } })
    }
  }
})
```

路由 meta 配置 keepAlive 缓存控制：

```ts
// 页面路由 meta 示例
defineOptions({
  meta: {
    title: 'route.userManagement', // i18n key
    cache: true,                    // 启用 keepAlive
  },
})
```

### API 层

API 按领域模块组织，统一使用 `api.post` 调用 gRPC HTTP 兼容路径：

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
// api/index.ts - Axios 实例与拦截器
import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_APP_API_BASEURL,
  timeout: 30000,
})

// 请求拦截：注入 Authorization
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截：统一错误处理
api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    if (error.response?.status === 401) {
      // Token 过期，清除状态并跳转登录
      useUserStore().clearLocalLoginState()
      router.push('/login')
    }
    return Promise.reject(error)
  },
)

export default api
```

::: tip API 规则
- 使用 `api.post` 而非 RESTful GET/PUT/DELETE
- body 字段使用 protobuf JSON 字段名
- 后端 `uint64` ID 在前端作为 `string` 处理
- `api-manifest.ts` 从后端 proto 自动生成，不要手动编辑
:::

### 国际化 (i18n)

使用 vue-i18n，翻译文件在 `src/i18n/package/` 下按语言组织：

```ts
// i18n/index.ts
import { createI18n } from 'vue-i18n'
import zhCN from './package/zh-CN'
import enUS from './package/en-US'

export type LocaleKey = 'zh-CN' | 'en'

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: initialLocale, // 从 localStorage 或 navigator.language 读取
  fallbackLocale: 'zh-CN',
  messages: { 'zh-CN': zhCN, en: enUS },
})
```

路由标题使用 i18n key，在 `App.vue` 中通过 `useTitle` + `t()` 动态设置页面标题。后端通过 `Accept-Language` 头传播语言偏好。

### 样式与主题

使用 SCSS 变量 + CSS 自定义属性实现主题：

```scss
// styles/resources/variables.scss - 设计令牌
$g-sidebar-width: 220px;
$g-header-height: 50px;

// styles/globals.scss - 全局样式入口
// resources 目录下的 SCSS 文件通过 vite.config.ts 自动注入所有组件
```

```vue
<!-- 组件样式 -->
<style lang="scss" scoped>
// 使用 BEM 命名
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

::: warning 样式规则
- 组件样式使用 `scoped` + SCSS
- 使用 BEM 命名约定：`.block__element--modifier`
- 全局变量通过 `styles/resources/` 自动注入，无需手动 import
- Element Plus 主题通过 ConfigProvider 配置语言、尺寸、按钮
:::

## 配置示例

### Vite 构建配置

| 配置项 | 值 | 说明 |
|--------|---|------|
| `base` | `'./'` | 相对路径部署 |
| `server.port` | `9000` | 开发服务器端口 |
| `server.proxy` | `/proxy` -> `VITE_APP_API_BASEURL` | API 代理 |
| `build.outDir` | `dist` / `dist-${mode}` | 输出目录 |
| `css.preprocessorOptions.scss.additionalData` | 自动注入 resources/ | 全局 SCSS 资源 |

### Vite 插件

| 插件 | 用途 |
|------|------|
| `@vitejs/plugin-vue` | Vue 3 SFC 支持 |
| `vite-plugin-pages` | 文件系统路由 |
| `vite-plugin-vue-meta-layouts` | 布局系统 |
| `unplugin-auto-import` | API 自动导入 (ref, computed 等) |
| `unplugin-vue-components` | 组件自动注册 |
| `vite-plugin-svg-icons-ng` | SVG 图标 |
| `vite-plugin-mock` | Mock 数据 |

### 环境变量

| 变量 | 说明 |
|------|------|
| `VITE_APP_API_BASEURL` | 后端 API 基础地址 |
| `VITE_OPEN_PROXY` | 是否开启代理 (`true`/`false`) |
| `VITE_BUILD_SOURCEMAP` | 是否生成 sourcemap |

### Lint 配置

| 工具 | 配置文件 | 关键规则 |
|------|---------|---------|
| ESLint | `.eslintrc` (oxlint) | `no-console: off`, `no-var: error`, `prefer-const: error`, `curly: always` |
| StyleLint | `.stylelintrc` | SCSS 扩展 + 标准规则 |
| Formatter | `vite.config.ts` fmt 字段 | 无分号, 单引号 |

### 常用脚本

| 命令 | 说明 |
|------|------|
| `pnpm dev` | 启动开发服务器 |
| `pnpm build` | 类型检查 + 构建 + 生成权限合约 |
| `pnpm lint` | 运行 lint 检查 |
| `pnpm format` | 格式化代码 |
| `pnpm type-check` | 仅 TypeScript 类型检查 |
| `pnpm new` | Plop 代码生成 |

## 实战示例

### 用户登录流程

完整登录流程涉及 API 调用、加密、Store 状态更新和路由跳转：

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
    // 1. 获取加密参数
    const cryptoRes = await apiUser.getLoginCrypto({
      username: loginForm.username,
      ua: navigator.userAgent,
    })

    // 2. 加密密码
    const payload = await encryptPasswordPayload(
      cryptoRes.data,
      loginForm.password,
    )

    // 3. 调用登录接口
    const res = await apiUser.login({
      username: loginForm.username,
      ...payload,
    })

    // 4. 保存 token 并初始化会话
    userStore.setToken(res.data.token)
    await userStore.bootstrapSession()

    // 5. 跳转到目标页或首页
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
      <el-input v-model="loginForm.username" placeholder="用户名" />
    </el-form-item>
    <el-form-item>
      <el-input v-model="loginForm.password" type="password" placeholder="密码" />
    </el-form-item>
    <el-button type="primary" :loading="loading" @click="handleLogin">
      登录
    </el-button>
  </el-form>
</template>
```

流程说明：先获取服务端加密参数（公钥 + keyId + challengeId），用公钥加密密码后提交。登录成功后保存 token，调用 `bootstrapSession` 获取用户信息与权限，最后根据 redirect 参数跳转。

### 列表页骨架屏 + 空状态

典型列表页包含加载骨架、空状态和数据展示三种状态：

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
    <!-- 骨架屏 -->
    <el-table v-if="loading && !tableData.length" :data="Array(5).fill({})">
      <el-table-column v-for="i in 4" :key="i">
        <template #default>
          <el-skeleton :rows="1" animated />
        </template>
      </el-table-column>
    </el-table>

    <!-- 空状态 -->
    <el-empty v-else-if="!tableData.length" description="暂无数据" />

    <!-- 数据表格 -->
    <el-table v-else :data="tableData">
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="name" label="姓名" />
      <el-table-column prop="phone" label="手机号" />
      <el-table-column label="操作">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleEdit(row)">编辑</el-button>
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

### Composable 封装权限判断

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
<!-- 在模板中使用 -->
<script setup lang="ts">
const { hasPermission } = useAuth()
</script>

<template>
  <el-button v-if="hasPermission('user:create')" type="primary">
    新建用户
  </el-button>
</template>
```

## 常见问题

**Q: `any` 类型是否可以使用？**

不可以。`any` 会绕过 TypeScript 类型检查，导致运行时错误无法在编译期发现。应使用具体类型、`unknown` + 类型守卫、或泛型约束。对于第三方库缺少类型的情况，使用 `declare module` 补充声明。

**Q: 为什么路由路径不能硬编码？**

硬编码路由路径（如 `router.push('/system/user')`）在路由重构时容易遗漏修改。应使用命名路由：`router.push({ name: 'SystemUser' })`。命名路由在路径变更时编译器可辅助检查。

**Q: API 调用没有错误处理怎么办？**

所有 API 调用必须有 `try/catch` 或 `.catch()` 处理。推荐使用 `loading` + `error` 状态模式：

```ts
const loading = ref(false)
const error = ref<string | null>(null)

async function fetchData() {
  loading.value = true
  error.value = null
  try {
    const res = await apiUser.getUserList(params)
    // 处理成功
  } catch (e) {
    error.value = (e as Error).message
    ElMessage.error('请求失败')
  } finally {
    loading.value = false
  }
}
```

**Q: 如何正确使用 keepAlive 缓存？**

在路由 meta 中设置 `cache: true`，`keepAlive` store 会自动管理缓存组件列表。页面退出时通过 `afterEach` 守卫清理。不要在组件内手动操作 `keep-alive` 的 include 列表。

**Q: 可以在组件中直接操作 DOM 吗？**

不可以。Vue 的响应式系统负责 DOM 更新，直接操作 DOM 会导致状态不一致。如果需要操作第三方 DOM（如图表库），使用 `ref` + `onMounted`/`watch` 在受控的生命周期钩子中进行。

**Q: 为什么不能在 library 代码中使用 `@/` 别名？**

`@/` 是 Vite 构建时解析的别名，仅适用于 `web/` 项目内部。如果代码被提取为独立库，别名将无法解析。library 代码应使用相对路径。

## 参考链接

- `web/tsconfig.json` - TypeScript 配置
- `web/vite.config.ts` - Vite 构建配置
- `web/src/main.ts` - 应用入口
- `web/src/App.vue` - 根组件（语言切换、热键、调试工具）
- `web/src/router/index.ts` - 路由守卫与动态路由
- `web/src/store/modules/` - Pinia Store 模块
- `web/src/api/modules/` - API 领域模块
- `web/src/api/index.ts` - Axios 实例与拦截器
- `web/src/api/api-manifest.ts` - 自动生成的 API 清单
- `web/src/i18n/index.ts` - 国际化配置
- `web/src/i18n/package/` - 翻译文件
- `web/src/composables/` - 组合式函数
- `web/src/styles/resources/` - 全局 SCSS 资源
- `web/src/layouts/` - 布局组件
- `web/package.json` - 依赖与脚本
