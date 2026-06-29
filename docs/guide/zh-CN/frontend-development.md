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

