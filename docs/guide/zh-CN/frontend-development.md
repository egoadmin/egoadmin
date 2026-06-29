# 前端开发

前端项目位于 `web/`，使用 Vue 3、TypeScript、Element Plus、Pinia、Vue Router 和 Vite。

## 目录结构

```text
web/src/
├── api/                    # API 实例、领域模块、api-manifest
├── assets/                 # 静态资源
├── components/             # 通用业务组件
├── config/                 # routeMenu 等配置
├── i18n/                   # 多语言
├── layouts/                # 布局
├── router/                 # 路由与守卫
├── store/                  # Pinia
├── styles/                 # 全局样式和主题
├── utils/                  # 通用工具
└── views/                  # 页面
```

## 启动和构建

```bash
cd web
pnpm install
pnpm run dev
pnpm run type-check
pnpm run build
```

主要脚本：

| 脚本 | 说明 |
|------|------|
| `dev` | 启动 Vite 开发服务器 |
| `type-check` | `vue-tsc --noEmit` |
| `build` | 类型检查 + 构建 + 生成权限合约 |
| `contract:gen` | 仅生成 `permission-contract.json` |
| `new` | plop 代码生成 |

## API 调用规则

业务 API 使用 gRPC HTTP 兼容路径：

```ts
api.post('/user.v1.UserService/GetUserList', {
  page: 1,
  limit: 20,
  username: '',
})
```

规则：

- 使用 `api.post`。
- 不使用 RESTful GET/PUT/DELETE。
- body 字段使用 protobuf JSON 字段名。
- 后端 `uint64` ID 在前端作为字符串处理。

```ts
export interface UserListRequest {
  page: number
  limit: number
  deptId?: string
  roleIds?: string[]
}
```

## 新增页面流程

1. 后端 proto 和 API 已可用。
2. 添加 API 模块。
3. 添加页面组件。
4. 添加 router module。
5. 添加 routeMenu 权限节点。
6. 添加 i18n 文案。
7. 运行 `pnpm run type-check` 和 `pnpm run build`。

### API 模块

```ts
// web/src/api/modules/role.ts
import api from '../index'

export interface GetRoleListRequest {
  page: number
  limit: number
  name?: string
  status?: number
}

export function getRoleList(data: GetRoleListRequest) {
  return api.post('/user.v1.RoleService/GetRoleList', data)
}
```

### 页面组件

```vue
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { getRoleList } from '@/api/modules/role'
import { useUserStore } from '@/store/modules/user'

const userStore = useUserStore()
const canAdd = computed(() => userStore.VA(['30302']))

const loading = ref(false)
const total = ref(0)
const tableData = ref<any[]>([])
const listParam = reactive({
  page: 1,
  limit: 20,
  name: '',
  status: 0,
})

async function loadData() {
  loading.value = true
  try {
    const res = await getRoleList(listParam)
    tableData.value = res.data?.list ?? []
    total.value = Number(res.data?.total ?? 0)
  } finally {
    loading.value = false
  }
}

onMounted(loadData)
</script>

<template>
  <PageMain>
    <SearchBar>
      <el-input v-model="listParam.name" clearable placeholder="角色名称" />
      <el-button type="primary" @click="loadData">查询</el-button>
      <el-button v-if="canAdd" type="primary">新增</el-button>
    </SearchBar>

    <el-table v-loading="loading" :data="tableData">
      <el-table-column prop="name" label="角色名称" />
      <el-table-column prop="code" label="角色编码" />
      <el-table-column prop="status" label="状态" />
    </el-table>
  </PageMain>
</template>
```

### 路由模块

```ts
// web/src/router/modules/role.ts
export default [
  {
    path: '/user/role',
    name: 'UserRole',
    component: () => import('@/views/user/role/index.vue'),
    meta: {
      title: '角色管理',
      cache: true,
    },
  },
]
```

### routeMenu 权限

```ts
{
  id: 30301,
  parentId: 30300,
  type: MenuType.Page,
  name: 'route.role',
  path: '/user/role',
  title: '角色管理',
  apis: [APIs.user.v1.RoleService.GetRoleList],
  children: [
    {
      id: 30302,
      type: MenuType.Button,
      title: '新增角色',
      apis: [APIs.user.v1.RoleService.AddRole],
    },
  ],
}
```

## 表单校验同步

前端表单规则要和 proto validate 标签一致：

```protobuf
string name = 1 [
  (tagger.tags) = "validate:\"required\" label:\"角色名称\""
];
```

```ts
const rules = {
  name: [{ required: true, message: '请输入角色名称', trigger: 'blur' }],
}
```

## 64 位 ID 规则

protobuf `uint64` / `fixed64` 在 JSON 中是字符串。

```ts
// 正确
const deptId = ref<string>('0')
const roleIds = ref<string[]>([])

// 错误
const deptId = ref<number>(0)
const roleIds = ref<number[]>([])
```

提交前检查：

```bash
rg -n "Number\\(|parseInt\\(|number\\[\\]|ref\\(0\\)" web/src
```

## 国际化

新增页面文案时同步 `web/src/i18n`：

```ts
export default {
  menu: {
    user: {
      role: '角色管理',
    },
  },
}
```

路由和菜单使用 locale key：

```ts
{
  title: '角色管理',
  locale: 'menu.user.role',
}
```

## 项目结构详解

前端项目位于 `web/`，入口文件为 `main.ts`，核心子目录如下：

```text
web/src/
├── api/                    # API 实例、领域模块、api-manifest
│   ├── api-manifest.ts     # 由 protoc-gen-api-catalog 生成，只读
│   ├── index.ts            # axios 实例、拦截器
│   └── modules/            # 按服务划分的 API 模块
├── assets/                 # 静态资源、精灵图
├── buildfile/              # plop 模板
├── components/             # 通用业务组件
├── config/                 # routeMenu 等配置
├── i18n/                   # 多语言
│   ├── index.ts            # vue-i18n 初始化
│   └── package/            # 语言包 (zh-CN.ts, en-US.ts)
├── layouts/                # 布局
├── menu/                   # 菜单配置
├── router/                 # 路由与守卫
│   └── modules/            # 按业务拆分的路由模块
├── store/                  # Pinia
│   └── modules/            # user, route, menu 等 store
├── styles/                 # 全局样式、主题、SCSS resources
├── types/                  # 全局类型声明
├── utils/                  # 工具函数
│   ├── login-crypto.ts     # 登录加密
│   └── heartbeat.ts        # 心跳保活
└── views/                  # 页面组件
```

::: tip
`api-manifest.ts` 是生成产物，不要手动编辑。后端 proto 变更后执行 `make gen` 和 `pnpm run build` 会自动更新。
:::

## TypeScript 配置要点

项目使用严格模式 TypeScript，关键配置如下：

```jsonc
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "paths": {
      "@/*": ["./src/*"],   // 源码别名
      "#/*": ["./src/types/*"]  // 类型别名
    },
    "types": [
      "element-plus/global",
      "vite-plugin-pages/client",
      "vite-plugin-vue-meta-layouts/client"
    ]
  }
}
```

- `strict: true` 开启所有严格检查，包括 `noImplicitAny`、`strictNullChecks`。
- `@/*` 映射到 `src/`，`#/*` 映射到 `src/types/`。
- `types` 数组注入 Element Plus 全局类型、文件路由类型和布局类型。

::: warning
不要在代码中使用 `any` 类型。如果确实需要，使用 `// eslint-disable-next-line` 并注明原因。
:::

## Vite 配置

### 插件链

`vite/plugins/index.ts` 按顺序加载插件：

| 插件 | 用途 |
|------|------|
| `@vitejs/plugin-vue` | Vue 3 SFC 支持 |
| `@vitejs/plugin-vue-jsx` | JSX/TSX 支持 |
| `@vitejs/plugin-legacy` | 旧浏览器兼容（仅 polyfill，不拆 chunk） |
| `vite-plugin-inspector` | 组件定位（开发模式） |
| `unplugin-auto-import` | 自动导入 Vue/Element Plus API |
| `unplugin-vue-components` | 按需自动注册组件 |
| `vite-plugin-svg-icons` | SVG 图标集 |
| `vite-plugin-mock-dev-server` | 开发模式 Mock |
| `vite-plugin-vue-meta-layouts` | 布局系统 |
| `vite-plugin-pages` | 基于文件的路由 |
| `vite-plugin-compression` | gzip/brotli 压缩（仅构建） |
| `vite-plugin-spritesmith` | CSS 精灵图生成 |

### 代理配置

开发服务器通过 `/proxy` 前缀转发请求到后端：

```ts
// vite.config.ts
server: {
  proxy: {
    '/proxy': {
      target: env.VITE_APP_API_BASEURL,
      changeOrigin: true,
      rewrite: (path: string) => path.replace(/\/proxy/, ''),
    },
  },
},
```

生产环境通过 Nginx 反向代理，不走 Vite 代理。

### SCSS 全局资源

`src/styles/resources/` 下的 `.scss` 文件会自动注入到每个组件的 `<style>` 中。精灵图目录下的 SCSS 也会自动加载。无需手动 `@import`。

## 组件开发模式

所有页面组件使用 `<script setup lang="ts">` 语法糖：

```vue
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

// 响应式状态
const loading = ref(false)
const formRef = ref<FormInstance>()

// 计算属性
const canEdit = computed(() => permissions.value.includes('EDIT'))

// 生命周期
onMounted(() => {
  loadData()
})
</script>
```

规则：

- 优先使用 `ref` 和 `reactive`，避免 Options API。
- 使用 `computed` 而非 watch 来派生状态。
- 表单 ref 命名 `formRef`，使用 `FormInstance` 类型。
- 组件自动注册（`unplugin-vue-components`），无需手动 import `PageMain`、`SearchBar` 等业务组件。

## API 层详解

### axios 实例 (`api/index.ts`)

核心拦截器逻辑：

```ts
const instance = axios.create({
  baseURL: import.meta.env.VITE_APP_API_BASEURL,
  timeout: 30000,
})

// 请求拦截：注入 token 和 accept-language
instance.interceptors.request.use((config) => {
  config.headers.Authorization = `Bearer ${getToken()}`
  config.headers['accept-language'] = getAcceptLanguage()
  return config
})

// 响应拦截：统一错误处理
instance.interceptors.response.use(
  (res) => res.data,
  (err) => {
    // UNAUTHENTICATED / LOGIN_EXPIRED → 自动 refresh token
    // 其他错误 → ElMessage 提示
  },
)
```

错误处理链：

1. 检测 gRPC 状态码（`UNAUTHENTICATED`、`LOGIN_EXPIRED`）。
2. 可刷新错误自动调用 refresh token 接口，成功后重试原请求。
3. 不可刷新错误弹出 `ElMessage` 错误提示。
4. 登录页和心跳错误静默处理，避免重复弹窗。

### api-manifest.ts

由 `protoc-gen-api-catalog` 从 OpenAPI spec 生成，导出 `APIs` 常量树：

```ts
export const APIs = {
  user: {
    v1: {
      RoleService: {
        AddRole: 'user.v1.RoleService/AddRole',
        GetRoleList: 'user.v1.RoleService/GetRoleList',
        // ...
      },
    },
  },
}
```

routeMenu 配置中通过 `APIs.user.v1.RoleService.AddRole` 绑定权限，构建时自动提取生成 `permission-contract.json`。

### API 模块编写规范

```ts
// web/src/api/modules/role.ts
import api from '../index'

export interface GetRoleListRequest {
  page: number
  limit: number
  name?: string
  status?: number
}

export interface RoleItem {
  id: string
  name: string
  code: string
  status: number
}

export function getRoleList(data: GetRoleListRequest) {
  return api.post('/user.v1.RoleService/GetRoleList', data)
}
```

- 每个文件对应一个后端服务。
- Request/Response 接口和调用函数写在同一个文件中。
- 路径使用 gRPC 兼容格式 `/package.Service/Method`。

## 国际化设置

项目使用 `vue-i18n` v9，支持 `zh-CN` 和 `en` 两种语言。

### 初始化

```ts
// i18n/index.ts
import { createI18n } from 'vue-i18n'
import zhCN from './package/zh-CN'
import enUS from './package/en-US'

const i18n = createI18n({
  legacy: false,
  locale: normalizeLocale(localStorage.getItem('egoadmin-locale')),
  fallbackLocale: 'zh-CN',
  messages: { 'zh-CN': zhCN, en: enUS },
})
```

语言选择持久化到 `localStorage`，key 为 `egoadmin-locale`。

### 语言包结构

```ts
// i18n/package/zh-CN.ts
export default {
  app: {
    home: '首页',
    login: '登录',
  },
  common: {
    add: '新增',
    edit: '编辑',
    delete: '删除',
  },
  menu: {
    user: {
      role: '角色管理',
    },
  },
}
```

### 使用方式

```vue
<template>
  <el-button>{{ t('common.add') }}</el-button>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>
```

路由和菜单标题使用 `locale` key，由布局组件自动翻译。

## 登录流程

登录采用 RSA-OAEP 加密方案，流程如下：

```text
1. 用户输入用户名和密码
2. 前端调用 GetLoginCrypto 获取公钥和 challenge
3. 前端用公钥加密 (用户名 + 密码 + challenge + nonce + 时间戳)
4. 前端将 passwordCipher + keyId + challengeId 发送到 Login 接口
5. 后端解密、验证 challenge 后签发 JWT
```

核心代码：

```ts
// utils/login-crypto.ts
export async function encryptPasswordPayload(
  payload: EncryptPasswordPayload,
): Promise<EncryptedPasswordPayload> {
  // 1. 获取加密参数
  const challenge = await apiUser.getLoginCrypto({
    username: payload.username,
    ua: payload.ua,
    action: payload.action,
  })

  // 2. 导入 RSA 公钥
  const key = await importPublicKey(challenge.publicKey)

  // 3. 加密 payload
  const plain = JSON.stringify({
    username: payload.username,
    password: payload.password,
    challengeId: challenge.challengeId,
    nonce: challenge.nonce,
    timestamp: Date.now(),
    ua: payload.ua,
    action: payload.action,
  })
  const cipher = await crypto.subtle.encrypt(
    { name: 'RSA-OAEP' },
    key,
    new TextEncoder().encode(plain),
  )

  // 4. 返回密文
  return {
    passwordCipher: arrayBufferToBase64(cipher),
    keyId: challenge.keyId,
    challengeId: challenge.challengeId,
  }
}
```

::: warning
密码绝不能明文传输。前端使用 `Web Crypto API` 的 `RSA-OAEP` + `SHA-256`，算法固定为 `RSA-OAEP-SHA256`。
:::

## 列表页模式

标准列表页由 `PageMain` + `SearchBar` + `ElTable` 组成：

```vue
<template>
  <PageMain>
    <!-- 搜索栏 -->
    <SearchBar>
      <el-input v-model="listParam.name" clearable placeholder="角色名称" />
      <el-select v-model="listParam.status" clearable placeholder="状态">
        <el-option label="启用" :value="1" />
        <el-option label="禁用" :value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">查询</el-button>
      <el-button v-if="canAdd" type="primary" @click="handleAdd">新增</el-button>
    </SearchBar>

    <!-- 数据表格 -->
    <el-table v-loading="loading" :data="tableData" border stripe>
      <el-table-column prop="name" label="角色名称" min-width="120" />
      <el-table-column prop="code" label="角色编码" min-width="120" />
      <el-table-column prop="status" label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'">
            {{ row.status === 1 ? '启用' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button v-if="canEdit" link type="primary" @click="handleEdit(row)">编辑</el-button>
          <el-button v-if="canDelete" link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <el-pagination
      v-model:current-page="listParam.page"
      v-model:page-size="listParam.limit"
      :total="total"
      layout="total, sizes, prev, pager, next, jumper"
      @change="loadData"
    />
  </PageMain>
</template>
```

列表页标准模式：

- `listParam` 使用 `reactive`，包含 `page`、`limit` 和搜索条件。
- `loadData` 函数在 `onMounted` 和查询按钮时调用。
- 权限按钮通过 `userStore.VA(['ID'])` 控制显隐。
- 使用 `el-pagination` 实现分页。

## 表单页模式

表单页使用 `ElForm` + 校验规则，通常在抽屉或弹窗中：

```vue
<script setup lang="ts">
import type { FormInstance, FormRules } from 'element-plus'

const formRef = ref<FormInstance>()
const formData = reactive({
  name: '',
  code: '',
  status: 1,
  remark: '',
})

// 校验规则与 proto validate 标签保持一致
const rules: FormRules = {
  name: [{ required: true, message: '请输入角色名称', trigger: 'blur' }],
  code: [
    { required: true, message: '请输入角色编码', trigger: 'blur' },
    { pattern: /^[A-Z_]+$/, message: '仅支持大写字母和下划线', trigger: 'blur' },
  ],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }],
}

async function handleSubmit() {
  await formRef.value?.validate()
  await addRole(formData)
  ElMessage.success('操作成功')
}
</script>

<template>
  <el-form ref="formRef" :model="formData" :rules="rules" label-width="100px">
    <el-form-item label="角色名称" prop="name">
      <el-input v-model="formData.name" placeholder="请输入角色名称" />
    </el-form-item>
    <el-form-item label="角色编码" prop="code">
      <el-input v-model="formData.code" placeholder="请输入角色编码" />
    </el-form-item>
    <el-form-item label="状态" prop="status">
      <el-radio-group v-model="formData.status">
        <el-radio :value="1">启用</el-radio>
        <el-radio :value="2">禁用</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="备注" prop="remark">
      <el-input v-model="formData.remark" type="textarea" :rows="3" />
    </el-form-item>
  </el-form>
</template>
```

表单规则要点：

- `required` 校验和 `trigger` 必须和 proto `validate` 标签一致。
- `prop` 属性必须和 `formData` 字段名一致。
- 提交前调用 `formRef.value?.validate()`，校验通过后再调 API。
- 编辑场景先回填数据，`formData` 使用 `Object.assign` 赋值。

## 常用前端命令

| 命令 | 说明 |
|------|------|
| `pnpm dev` | 启动开发服务器（HMR 热更新） |
| `pnpm build` | 类型检查 + 构建 + 生成权限合约 |
| `pnpm type-check` | 仅 `vue-tsc --noEmit` 类型检查 |
| `pnpm lint` | 运行 linter 检查 |
| `pnpm contract:gen` | 仅生成 `permission-contract.json` |
| `pnpm new` | plop 代码生成器 |

```bash
# 开发调试
cd web && pnpm dev

# 提交前验证
cd web && pnpm type-check && pnpm build

# 完整构建（CI 使用）
cd web && pnpm build
```

## 验证

```bash
cd web
pnpm run type-check
pnpm run contract:gen
pnpm run build
```

权限相关页面还需要手工验证：

- admin 可见全部菜单和按钮。
- 普通用户只看到已授权菜单和按钮。
- 隐藏按钮对应的后端 API 直接调用也被拒绝。

## 延伸阅读

- [API 开发工作流](./api-development.md) -- 后端 API 从 proto 到前端的完整链路。
- [路由与菜单](./frontend-routing.md) -- 路由配置和菜单权限绑定。
- [组件库](./frontend-components.md) -- 业务组件使用指南。

