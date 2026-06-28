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
