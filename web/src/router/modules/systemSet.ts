import type { RouteRecordRaw } from 'vue-router'
import { routeTitle } from '@/utils/i18n'

function Layout() {
  return import('@/layouts/index.vue')
}

const routes: RouteRecordRaw = {
  path: '/system',
  component: Layout,
  redirect: '/system/role',
  name: 'System',
  meta: {
    title: routeTitle('menu.systemSettings'),
    icon: 'setting',
  },
  children: [
    {
      path: 'role',
      name: 'Role',
      component: () => import('@/views/system/role/index.vue'),
      meta: {
        title: routeTitle('menu.roleManagement'),
      },
    },
    {
      path: 'online_user',
      name: 'OnlineUser',
      component: () => import('@/views/system/online_user/index.vue'),
      meta: {
        title: routeTitle('menu.onlineUsers'),
      },
    },
    {
      path: 'operation_log',
      name: 'OperationLog',
      component: () => import('@/views/system/operation_log/index.vue'),
      meta: {
        title: routeTitle('menu.operationLogs'),
      },
    },
  ],
}

export default routes
