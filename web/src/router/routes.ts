import { setupLayouts } from 'virtual:meta-layouts'
import generatedRoutes from 'virtual:generated-pages'
import type { RouteRecordRaw } from 'vue-router'

// import MultilevelMenuExample from './modules/multilevel.menu.example'
// import BreadcrumbExample from './modules/breadcrumb.example'
import SystemSet from './modules/systemSet'
import UserManagement from './modules/userManagement'
import type { Route } from '#/global'
import { routeTitle } from '@/utils/i18n'

// 固定路由（默认路由）
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
    // 落地页：壳外独立公开页（无需登录）
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
        // 未登录访问首页展示介绍页；已登录默认进入角色管理（去掉后台壳内首页）
        redirect: () => (localStorage.getItem('token') ? '/system/role' : { name: 'landing' }),
        meta: {
          title: routeTitle('app.home'),
          breadcrumb: false,
        },
      },
      {
        path: 'reload',
        name: 'reload',
        component: () => import('@/views/reload.vue'),
        meta: {
          title: routeTitle('menu.reload'),
          breadcrumb: false,
        },
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

// 系统路由
const systemRoutes: RouteRecordRaw[] = [
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
        name: 'home',
        // 未登录访问首页展示介绍页；已登录默认进入角色管理（去掉后台壳内首页）
        redirect: () => (localStorage.getItem('token') ? '/system/role' : { name: 'landing' }),
        meta: {
          title: routeTitle('app.home'),
          breadcrumb: false,
        },
      },
      {
        path: 'reload',
        name: 'reload',
        component: () => import('@/views/reload.vue'),
        meta: {
          title: routeTitle('menu.reload'),
          breadcrumb: false,
        },
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

// 动态路由（异步路由、导航栏路由）
const asyncRoutes: Route.recordMainRaw[] = [
  {
    meta: {
      title: routeTitle('menu.demo'),
      icon: 'sidebar-default',
    },
    children: [
      // MultilevelMenuExample,
      // BreadcrumbExample,
      SystemSet,
      UserManagement,
    ],
  },
]

const constantRoutesByFilesystem = generatedRoutes.filter((item) => {
  return item.meta?.enabled !== false && item.meta?.constant === true
})

const asyncRoutesByFilesystem = setupLayouts(generatedRoutes.filter((item) => {
  return item.meta?.enabled !== false && item.meta?.constant !== true && item.meta?.layout !== false
}))

export {
  constantRoutes,
  systemRoutes,
  asyncRoutes,
  constantRoutesByFilesystem,
  asyncRoutesByFilesystem,
}
