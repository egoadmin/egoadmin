# 前端路由与权限

EgoAdmin 基于 Vue Router 实现双轨路由体系，通过后端菜单 API 驱动动态路由生成，配合权限控制实现页面级访问管理。

## 概述

路由系统采用**固定路由 + 动态路由**双轨模式：

- **固定路由（constantRoutes）**：登录页、404、落地页等无需权限的页面，在应用初始化时即注册。
- **动态路由（asyncRoutes）**：由后端菜单 API 返回的权限数据驱动，在用户登录后通过 `router.addRoute()` 动态注入。

整个流程由 `router/index.ts` 中的全局守卫 `beforeEach` 统一调度。

## 核心用法

### 路由定义

每个业务模块在 `router/modules/` 下维护独立路由文件，导出一个 `RouteRecordRaw` 对象：

```typescript
// router/modules/userManagement.ts
import type { RouteRecordRaw } from 'vue-router'
import { routeTitle } from '@/utils/i18n'

function Layout() {
  return import('@/layouts/index.vue')
}

const routes: RouteRecordRaw = {
  path: '/user_management',
  component: Layout,
  redirect: '/user_management/user',
  name: 'UserManagement',
  meta: {
    title: routeTitle('menu.userManagement'),
    icon: 'user-circle',
  },
  children: [
    {
      path: 'user',
      name: 'User',
      component: () => import('@/views/userManagement/user/index.vue'),
      meta: {
        title: routeTitle('menu.userList'),
      },
    },
    {
      path: 'organization',
      name: 'Organization',
      component: () => import('@/views/userManagement/organization/index.vue'),
      meta: {
        title: routeTitle('menu.organizationManagement'),
      },
    },
  ],
}

export default routes
```

::: tip 路由命名
所有路由必须设置 `name` 属性，动态路由在登出时需要通过 `name` 来精确移除。
:::

### 固定路由

固定路由定义在 `router/routes.ts`，包含以下类别：

```typescript
// router/routes.ts
const constantRoutes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/login.vue'),
    meta: {
      title: routeTitle('app.login'),
    },
  },
  {
    path: '/welcome',
    name: 'landing',
    component: () => import('@/views/landing.vue'),
    meta: {
      title: routeTitle('app.landing'),
      constant: true,
    },
  },
  {
    path: '/:all(.*)*',
    name: 'notFound',
    component: () => import('@/views/[...all].vue'),
    meta: {
      title: routeTitle('app.notFoundTitle'),
    },
  },
  {
    path: '/',
    component: () => import('@/layouts/index.vue'),
    meta: {
      title: routeTitle('app.home'),
      breadcrumb: false,
    },
    children: [
      {
        path: '',
        name: 'Home',
        redirect: () =>
          localStorage.getItem('token') ? '/system/role' : { name: 'landing' },
      },
      {
        path: 'reload',
        name: 'reload',
        component: () => import('@/views/reload.vue'),
        meta: { title: routeTitle('menu.reload'), breadcrumb: false },
      },
      {
        path: 'setting',
        name: 'personalSetting',
        component: () => import('@/views/personal/setting.vue'),
        meta: {
          title: routeTitle('menu.personalSetting'),
          cache: 'personalEditPassword',
        },
      },
    ],
  },
]
```

### 路由守卫流程

守卫逻辑位于 `router/index.ts`，核心流程如下：

```text
beforeEach(to, from, next)
  |
  +-- 已登录?
  |     |
  |     +-- 动态路由已生成?
  |     |     |
  |     |     +-- 是 --> 更新导航选中状态 --> 放行
  |     |     |
  |     |     +-- 否 --> 生成动态路由 --> 注册路由 --> 重新进入
  |     |
  |     +-- 访问登录页 --> 重定向到首页
  |
  +-- 未登录?
        |
        +-- 公开页(login, landing) --> 放行
        |
        +-- 其他页面 --> 重定向到登录页
```

实际守卫代码：

```typescript
// router/index.ts（核心逻辑）
router.beforeEach(async (to, from, next) => {
  const userStore = useUserStore()
  const routeStore = useRouteStore()
  const permissionStore = usePermissionStore()

  if (userStore.isLogin) {
    if (routeStore.isGenerate) {
      // 动态路由已就绪，正常导航
      if (to.name === 'login') {
        next({ name: 'home', replace: true })
      } else {
        ;!to.name || to.name === 'notFound'
          ? next({ path: '/' })
          : next()
      }
    } else {
      // 1. 获取权限
      await userStore.getPermissions()
      // 2. 根据权限生成动态路由
      await permissionStore.setRoutes(userStore.menus)
      // 3. 注册路由到 Router 实例
      const removeRoutes: Function[] = []
      routeStore.flatRoutes.forEach((route) => {
        if (!/^(https?:|mailto:|tel:)/.test(route.path)) {
          removeRoutes.push(router.addRoute(route as RouteRecordRaw))
        }
      })
      routeStore.setCurrentRemoveRoutes(removeRoutes)
      // 4. 重新进入当前路由（replace 避免历史重复）
      next({ path: to.path, query: to.query, replace: true })
    }
  } else {
    const publicRoutes = ['login', 'landing']
    if (publicRoutes.includes(to.name as string)) {
      next()
    } else {
      next({ name: 'login', replace: true })
    }
  }
})
```

::: warning 动态路由生成时机
动态路由只在首次导航时生成一次（`routeStore.isGenerate` 标记），后续导航直接放行。如果遇到 404 问题，请检查守卫中动态路由是否正确注册。
:::

### 动态路由生成

动态路由由 `permissionStore.setRoutes()` 生成，核心逻辑：

```typescript
// store/modules/permission.ts
async function setRoutes(roles: any) {
  let accessedRoutes
  const rolesArr = typeof roles !== 'object' ? roles.split(',') : roles

  // admin/root 用户拥有全部路由
  if (userStore.username === 'admin' || userStore.username === 'root') {
    accessedRoutes = asyncRoutes[0].children
  } else {
    // 根据后端返回的权限 ID 过滤路由
    accessedRoutes = await filterAsyncRoutes(menu, rolesArr)
  }

  dynamicRoutes.value = cloneDeep(accessedRoutes)
}
```

权限过滤通过 `routeMenu.ts` 配置的权限树与用户权限列表做交集，只有匹配的路由才会被注入。

### 路由权限配置

路由权限映射维护在 `config/routeMenu.ts`，将路由路径映射到后端权限 ID：

```typescript
// config/routeMenu.ts（示意）
export const menu = [
  {
    id: '1',
    path: '/system',
    child: [
      { id: '1-1', path: 'role' },
      { id: '1-2', path: 'menu' },
      { id: '1-3', path: 'user' },
    ],
  },
  {
    id: '2',
    path: '/user_management',
    child: [
      { id: '2-1', path: 'user' },
      { id: '2-2', path: 'organization' },
    ],
  },
]
```

### KeepAlive 缓存控制

页面缓存通过 `meta.cache` 控制，在 `router.afterEach` 中自动管理：

```typescript
// router/index.ts - afterEach
router.afterEach((to, from) => {
  const keepAliveStore = useKeepAliveStore()

  // 进入页面：如果 meta.cache 为 truthy，将组件名加入缓存列表
  if (to.meta.cache) {
    const componentName = to.matched.at(-1)?.components?.default.name
    if (componentName) {
      keepAliveStore.add(componentName)
    }
  }

  // 离开页面：根据缓存规则决定是否移除
  if (from.meta.cache) {
    const componentName = from.matched.at(-1)?.components?.default.name
    if (componentName) {
      switch (typeof from.meta.cache) {
        case 'string':
          // cache: 'pageName' -- 仅在跳转到指定页面时保留缓存
          if (from.meta.cache !== to.name) {
            keepAliveStore.remove(componentName)
          }
          break
        case 'object':
          // cache: ['pageA', 'pageB'] -- 跳转到列表中的页面时保留缓存
          if (!from.meta.cache.includes(to.name as string)) {
            keepAliveStore.remove(componentName)
          }
          break
      }
      // 进入 reload 页面时清除所有缓存
      if (to.name === 'reload') {
        keepAliveStore.remove(componentName)
      }
    }
  }
})
```

`meta.cache` 支持以下取值：

| 值 | 行为 | 示例 |
|---|---|---|
| `true` | 始终缓存 | 列表页 |
| `string` | 仅跳转到指定页面时保留缓存 | `cache: 'UserEdit'` |
| `string[]` | 跳转到列表中任意页面时保留缓存 | `cache: ['Detail', 'Edit']` |

::: warning 组件名必须匹配
KeepAlive 依赖组件的 `name` 属性匹配缓存。如果页面组件未设置 `name`，缓存将失效，控制台会输出警告。
:::

## 配置示例

### 完整路由模块示例

```typescript
// router/modules/systemSet.ts
import type { RouteRecordRaw } from 'vue-router'
import { routeTitle } from '@/utils/i18n'

function Layout() {
  return import('@/layouts/index.vue')
}

const routes: RouteRecordRaw = {
  path: '/system',
  component: Layout,
  redirect: '/system/role',
  name: 'SystemSet',
  meta: {
    title: routeTitle('menu.systemManagement'),
    icon: 'settings',
  },
  children: [
    {
      path: 'role',
      name: 'Role',
      component: () => import('@/views/systemSet/role/index.vue'),
      meta: {
        title: routeTitle('menu.roleManagement'),
        cache: true,
      },
    },
    {
      path: 'role/edit/:id?',
      name: 'RoleEdit',
      component: () => import('@/views/systemSet/role/edit.vue'),
      meta: {
        title: routeTitle('menu.roleEdit'),
        hidden: true,
        cache: 'Role',
      },
    },
    {
      path: 'menu',
      name: 'SystemMenu',
      component: () => import('@/views/systemSet/menu/index.vue'),
      meta: {
        title: routeTitle('menu.menuManagement'),
      },
    },
  ],
}

export default routes
```

### 路由 meta 完整字段

```typescript
// 扩展的 RouteMeta 类型
interface RouteMeta {
  // 页面标题（i18n key 或直接字符串）
  title: string | (() => string)
  // KeepAlive 缓存控制
  cache?: boolean | string | string[]
  // 在侧边栏中隐藏
  hidden?: boolean
  // 侧边栏图标
  icon?: string
  // 面包屑显示控制
  breadcrumb?: boolean
  // 固定路由（无需登录）
  constant?: boolean
  // 侧边栏是否显示
  sidebar?: boolean
  // 默认展开
  defaultOpened?: boolean
  // 权限标识
  auth?: string | string[]
  // 布局是否启用
  layout?: boolean
}
```

## 实际场景

### 场景一：新增业务模块

1. 在 `router/modules/` 下创建路由文件：

```typescript
// router/modules/order.ts
import type { RouteRecordRaw } from 'vue-router'
import { routeTitle } from '@/utils/i18n'

function Layout() {
  return import('@/layouts/index.vue')
}

const routes: RouteRecordRaw = {
  path: '/order',
  component: Layout,
  redirect: '/order/list',
  name: 'OrderManagement',
  meta: {
    title: routeTitle('menu.orderManagement'),
    icon: 'shopping-cart',
  },
  children: [
    {
      path: 'list',
      name: 'OrderList',
      component: () => import('@/views/order/list.vue'),
      meta: {
        title: routeTitle('menu.orderList'),
        cache: true,
      },
    },
    {
      path: 'detail/:id',
      name: 'OrderDetail',
      component: () => import('@/views/order/detail.vue'),
      meta: {
        title: routeTitle('menu.orderDetail'),
        cache: 'OrderList',
      },
    },
  ],
}

export default routes
```

2. 在 `router/routes.ts` 的 `asyncRoutes` 中注册：

```typescript
import OrderManagement from './modules/order'

const asyncRoutes: Route.recordMainRaw[] = [
  {
    meta: { title: routeTitle('menu.demo'), icon: 'sidebar-default' },
    children: [
      SystemSet,
      UserManagement,
      OrderManagement, // 新增
    ],
  },
]
```

3. 在 `config/routeMenu.ts` 中添加权限节点。

### 场景二：页面跳转

```typescript
import { useRouter } from 'vue-router'

const router = useRouter()

// 始终使用 name 进行导航，不要使用 path 字符串
router.push({ name: 'OrderDetail', params: { id: '123' } })

// 带查询参数
router.push({
  name: 'OrderList',
  query: { status: 'pending', page: '1' },
})

// 返回上一页
router.back()
```

::: danger 禁止使用 path 导航
必须使用 `name` 进行路由导航，不要使用 `path` 字符串拼接。动态路由使用 `addRoute` 注册后路径可能与预期不同，使用 `name` 能确保导航准确性。
:::

### 场景三：控制台调试路由

```typescript
// 在浏览器控制台查看当前路由
console.log(window.__ROUTER__.currentRoute)

// 查看所有已注册路由
console.log(window.__ROUTER__.getRoutes())

// 手动导航
window.__ROUTER__.push({ name: 'Role' })
```

## 工作原理

### 路由注册时序

```text
应用启动
  |
  +-- createRouter(constantRoutes)        // 注册固定路由
  |
  +-- beforeEach 首次触发
        |
        +-- userStore.getPermissions()    // 请求后端菜单 API
        |
        +-- permissionStore.setRoutes()   // 过滤出用户可访问路由
        |
        +-- router.addRoute(route)        // 逐条注入动态路由
        |
        +-- next({ path: to.path, replace: true })  // 重新进入
```

### 面包屑生成

面包屑数据通过路由的 `meta.breadcrumbNeste` 自动生成，在 `afterEach` 中由 Vue Router 的 `matched` 数组推导：

```typescript
// afterEach 中设置页面标题
settingsStore.setTitle(
  to.meta.breadcrumbNeste?.at(-1)?.title ?? to.meta.title
)
```

### 多主导航模式

当 `settings.menu.menuMode` 不为 `single` 时，主导航切换需要根据当前路由路径定位：

```typescript
// 非 single 模式下更新主导航选中状态
if (settingsStore.settings.menu.menuMode !== 'single') {
  menuStore.setActived(to.path)
}
```

## 常见问题

### 登录后出现 404

**原因**：动态路由尚未注册完成，守卫直接放行了目标路由。

**排查步骤**：

1. 打开控制台，检查 `permissionStore.dynamicRoutes` 是否为空。
2. 检查后端菜单 API 返回的权限 ID 是否与 `routeMenu.ts` 中的配置匹配。
3. 确认路由模块已正确导入并在 `asyncRoutes` 中注册。

### 刷新后页面白屏

**原因**：动态路由在刷新后丢失，守卫需要重新生成。

**排查步骤**：

1. 确认 `localStorage` 中 `token` 和 `menus` 是否存在。
2. 检查 `userStore.getPermissions()` 是否正常返回。
3. 确认守卫中 `routeStore.isGenerate` 的初始值为 `false`。

### KeepAlive 不生效

**原因**：页面组件未设置 `name` 属性，或 `meta.cache` 配置错误。

**解决方案**：

```vue
<script lang="ts">
export default {
  name: 'UserList', // 必须与 KeepAlive 的 include 匹配
}
</script>
```

### 动态路由未完全清除

**原因**：路由缺少 `name` 属性，无法通过 `removeRoute` 精确移除。

**解决方案**：确保所有动态路由都设置了唯一的 `name`。

## 参考链接

- [Vue Router 官方文档](https://router.vuejs.org/zh/)
- [Vue Router 导航守卫](https://router.vuejs.org/zh/guide/advanced/navigation-guards.html)
- [前端开发总览](./frontend-development.md)
- [状态管理](./state-management.md)
- [权限系统](../permission-system.md)
