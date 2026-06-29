# Frontend Routing and Permissions

EgoAdmin implements a dual-track routing system based on Vue Router, driven by a backend menu API for dynamic route generation and integrated with permission control for page-level access management.

## Overview

The routing system uses a **fixed routes + dynamic routes** dual-track model:

- **Fixed routes (constantRoutes)**: Login, 404, landing page, and other permission-free pages are registered at application startup.
- **Dynamic routes (asyncRoutes)**: Driven by backend menu API permission data, dynamically injected via `router.addRoute()` after user login.

The entire flow is orchestrated by the global `beforeEach` guard in `router/index.ts`.

## Core Usage

### Route Definition

Each business module maintains its own route file under `router/modules/`, exporting a `RouteRecordRaw` object:

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

::: tip Route Naming
All routes must have a `name` property. Dynamic routes need names for precise removal during logout.
:::

### Fixed Routes

Fixed routes are defined in `router/routes.ts` and include the following categories:

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

### Route Guard Flow

The guard logic is in `router/index.ts`. The core flow:

```text
beforeEach(to, from, next)
  |
  +-- Logged in?
  |     |
  |     +-- Dynamic routes generated?
  |     |     |
  |     |     +-- Yes --> Update nav active state --> Proceed
  |     |     |
  |     |     +-- No  --> Generate dynamic routes --> Register --> Re-enter
  |     |
  |     +-- Navigating to login page --> Redirect to home
  |
  +-- Not logged in?
        |
        +-- Public page (login, landing) --> Proceed
        |
        +-- Other pages --> Redirect to login
```

Actual guard code:

```typescript
// router/index.ts (core logic)
router.beforeEach(async (to, from, next) => {
  const userStore = useUserStore()
  const routeStore = useRouteStore()
  const permissionStore = usePermissionStore()

  if (userStore.isLogin) {
    if (routeStore.isGenerate) {
      // Dynamic routes ready, navigate normally
      if (to.name === 'login') {
        next({ name: 'home', replace: true })
      } else {
        ;!to.name || to.name === 'notFound'
          ? next({ path: '/' })
          : next()
      }
    } else {
      // 1. Fetch permissions
      await userStore.getPermissions()
      // 2. Generate dynamic routes based on permissions
      await permissionStore.setRoutes(userStore.menus)
      // 3. Register routes to the Router instance
      const removeRoutes: Function[] = []
      routeStore.flatRoutes.forEach((route) => {
        if (!/^(https?:|mailto:|tel:)/.test(route.path)) {
          removeRoutes.push(router.addRoute(route as RouteRecordRaw))
        }
      })
      routeStore.setCurrentRemoveRoutes(removeRoutes)
      // 4. Re-enter current route (replace avoids duplicate history)
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

::: warning Dynamic Route Generation Timing
Dynamic routes are generated only once on the first navigation (`routeStore.isGenerate` flag). Subsequent navigations proceed directly. If you encounter 404 issues, check that dynamic routes are correctly registered in the guard.
:::

### Dynamic Route Generation

Dynamic routes are generated by `permissionStore.setRoutes()`:

```typescript
// store/modules/permission.ts
async function setRoutes(roles: any) {
  let accessedRoutes
  const rolesArr = typeof roles !== 'object' ? roles.split(',') : roles

  // admin/root users get all routes
  if (userStore.username === 'admin' || userStore.username === 'root') {
    accessedRoutes = asyncRoutes[0].children
  } else {
    // Filter routes by permission IDs from the backend
    accessedRoutes = await filterAsyncRoutes(menu, rolesArr)
  }

  dynamicRoutes.value = cloneDeep(accessedRoutes)
}
```

Permission filtering intersects the permission tree configured in `routeMenu.ts` with the user's permission list. Only matching routes are injected.

### Route Permission Configuration

Route permission mappings are maintained in `config/routeMenu.ts`, mapping route paths to backend permission IDs:

```typescript
// config/routeMenu.ts (illustrative)
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

### KeepAlive Cache Control

Page caching is controlled by `meta.cache` and managed automatically in `router.afterEach`:

```typescript
// router/index.ts - afterEach
router.afterEach((to, from) => {
  const keepAliveStore = useKeepAliveStore()

  // Entering page: if meta.cache is truthy, add component name to cache list
  if (to.meta.cache) {
    const componentName = to.matched.at(-1)?.components?.default.name
    if (componentName) {
      keepAliveStore.add(componentName)
    }
  }

  // Leaving page: decide whether to remove based on cache rules
  if (from.meta.cache) {
    const componentName = from.matched.at(-1)?.components?.default.name
    if (componentName) {
      switch (typeof from.meta.cache) {
        case 'string':
          // cache: 'pageName' -- keep cache only when navigating to that page
          if (from.meta.cache !== to.name) {
            keepAliveStore.remove(componentName)
          }
          break
        case 'object':
          // cache: ['pageA', 'pageB'] -- keep cache when navigating to listed pages
          if (!from.meta.cache.includes(to.name as string)) {
            keepAliveStore.remove(componentName)
          }
          break
      }
      // Clear cache when entering reload page
      if (to.name === 'reload') {
        keepAliveStore.remove(componentName)
      }
    }
  }
})
```

`meta.cache` supports these values:

| Value | Behavior | Example |
|---|---|---|
| `true` | Always cached | List pages |
| `string` | Keep cache only when navigating to the specified page | `cache: 'UserEdit'` |
| `string[]` | Keep cache when navigating to any listed page | `cache: ['Detail', 'Edit']` |

::: warning Component Name Must Match
KeepAlive matches cached components by their `name` property. If a page component does not have `name` set, caching will silently fail and a warning will be logged to the console.
:::

## Configuration Examples

### Complete Route Module Example

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

### Complete Route Meta Fields

```typescript
// Extended RouteMeta type
interface RouteMeta {
  // Page title (i18n key or direct string)
  title: string | (() => string)
  // KeepAlive cache control
  cache?: boolean | string | string[]
  // Hide from sidebar
  hidden?: boolean
  // Sidebar icon
  icon?: string
  // Breadcrumb display control
  breadcrumb?: boolean
  // Fixed route (no login required)
  constant?: boolean
  // Whether to show in sidebar
  sidebar?: boolean
  // Default expanded
  defaultOpened?: boolean
  // Permission identifier
  auth?: string | string[]
  // Whether layout is enabled
  layout?: boolean
}
```

## Real-World Examples

### Example 1: Adding a Business Module

1. Create a route file under `router/modules/`:

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

2. Register in `router/routes.ts` within `asyncRoutes`:

```typescript
import OrderManagement from './modules/order'

const asyncRoutes: Route.recordMainRaw[] = [
  {
    meta: { title: routeTitle('menu.demo'), icon: 'sidebar-default' },
    children: [
      SystemSet,
      UserManagement,
      OrderManagement, // new
    ],
  },
]
```

3. Add permission nodes in `config/routeMenu.ts`.

### Example 2: Page Navigation

```typescript
import { useRouter } from 'vue-router'

const router = useRouter()

// Always use name-based navigation, never path strings
router.push({ name: 'OrderDetail', params: { id: '123' } })

// With query parameters
router.push({
  name: 'OrderList',
  query: { status: 'pending', page: '1' },
})

// Go back
router.back()
```

::: danger Do Not Use Path Navigation
Always use `name` for route navigation. Do not concatenate `path` strings. Dynamic routes registered via `addRoute` may have different paths than expected; using `name` ensures navigation accuracy.
:::

### Example 3: Debugging Routes in the Console

```typescript
// Check current route in browser console
console.log(window.__ROUTER__.currentRoute)

// View all registered routes
console.log(window.__ROUTER__.getRoutes())

// Manual navigation
window.__ROUTER__.push({ name: 'Role' })
```

## How It Works

### Route Registration Timeline

```text
App startup
  |
  +-- createRouter(constantRoutes)        // Register fixed routes
  |
  +-- beforeEach first trigger
        |
        +-- userStore.getPermissions()    // Request backend menu API
        |
        +-- permissionStore.setRoutes()   // Filter accessible routes
        |
        +-- router.addRoute(route)        // Inject dynamic routes one by one
        |
        +-- next({ path: to.path, replace: true })  // Re-enter
```

### Breadcrumb Generation

Breadcrumb data is auto-generated from route `meta.breadcrumbNeste`, derived from Vue Router's `matched` array in `afterEach`:

```typescript
// Set page title in afterEach
settingsStore.setTitle(
  to.meta.breadcrumbNeste?.at(-1)?.title ?? to.meta.title
)
```

### Multi-Main-Nav Mode

When `settings.menu.menuMode` is not `single`, the main navigation active state is located by the current route path:

```typescript
// Update main nav active state in non-single mode
if (settingsStore.settings.menu.menuMode !== 'single') {
  menuStore.setActived(to.path)
}
```

## Common Issues

### 404 After Login

**Cause**: Dynamic routes have not finished registering; the guard lets the target route through prematurely.

**Troubleshooting**:

1. Open the console and check if `permissionStore.dynamicRoutes` is empty.
2. Verify that backend menu API permission IDs match the configuration in `routeMenu.ts`.
3. Confirm that route modules are correctly imported and registered in `asyncRoutes`.

### Blank Screen After Refresh

**Cause**: Dynamic routes are lost after refresh and the guard needs to regenerate them.

**Troubleshooting**:

1. Confirm `token` and `menus` exist in `localStorage`.
2. Check that `userStore.getPermissions()` returns successfully.
3. Verify that `routeStore.isGenerate` starts as `false` in the guard.

### KeepAlive Not Working

**Cause**: Page component does not have a `name` property, or `meta.cache` is misconfigured.

**Solution**:

```vue
<script lang="ts">
export default {
  name: 'UserList', // Must match KeepAlive's include
}
</script>
```

### Dynamic Routes Not Fully Cleared

**Cause**: Routes lack the `name` property, preventing precise removal via `removeRoute`.

**Solution**: Ensure all dynamic routes have a unique `name`.

## Reference Links

- [Vue Router Official Documentation](https://router.vuejs.org/)
- [Vue Router Navigation Guards](https://router.vuejs.org/guide/advanced/navigation-guards.html)
- [Frontend Development Overview](./frontend-development.md)
- [State Management](./state-management.md)
- [Permission System](../permission-system.md)
