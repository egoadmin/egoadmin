import type { RouteRecordRaw } from 'vue-router'
import useUserStore from './user'
import { asyncRoutes } from '@/router/routes'
import { menu } from '@/config/routeMenu'
import { cloneDeep } from 'lodash-es'

export interface IPermissionState {
  routes: RouteRecordRaw[]
  dynamicRoutes: RouteRecordRaw[]
}

const usePermissionStore = defineStore(
  // 唯一ID
  'permission',
  () => {
    const userStore = useUserStore()
    const routes = ref([])
    const dynamicRoutes = ref([])
    function hasPermission(roles: string[], routes: RouteRecordRaw | any) {
      const data: any = []
      // let ant = false
      // 传入动态路由的route routes参数注意动态路由结构问题
      const route = [...routes[0].children]
      handleRoutes(route)
      route.forEach((item: any) => {
        roles.forEach((val: any) => {
          if (val === item.path) {
            data.push(item)
          }
        })
      })
      const dataList = JSON.parse(JSON.stringify(data))

      // let swte: any;
      dataList?.forEach((item: any, index: number) => {
        item.children?.forEach((dataAnt: any) => {
          // console.log(!roles.includes(dataant.path), dataant.meta.judge, dataant.meta.title);
          if (!roles.includes(dataAnt.path)) {
            data[index]?.children?.forEach((ant: any, sun: number) => {
              if (dataAnt.path === ant.path) {
                data[index]?.children.splice(sun, 1)
              }
            })
          }
        })
        item.children = Array.from(new Set(item?.children))
        // console.log(data[index].children[swte]);
        // data[index].children.splice(swte, 1)
      })
      // data.push(asyncRoutes[index])
      console.log(data)

      return data
    }

    function filterAsyncRoutes(routes: any, roles: any) {
      // 用于保存可以显示的路径
      const res: string[] = []
      // 根据角色权限过滤出可以显示的路径
      const filterPath = (childs: any) => {
        childs.forEach((item: any) => {
          // 顶层路由拥有路由判断
          roles?.forEach((data: string) => {
            if (`${data}` === `${item.id}` && item.path) {
              res.push(item.path)
            }
          })
          if (item.child) {
            filterPath(item.child)
          }
        })
      }
      filterPath(routes)
      routes.forEach((item: any) => {
        const possessorId = item.child?.map((row: any) => `${row.path}`) || []
        const parentPath = item.path
        res.forEach((data: any) => {
          if (possessorId.includes(data)) {
            res.push(parentPath)
          }
        })
      })

      // 去重后的路径列表
      const pathList = Array.from(new Set(res))
      // console.log(value, '一级路由');
      // 特殊情况，课程表需要考试详情和试卷预览
      return hasPermission(pathList, asyncRoutes)
    }

    async function setRoutes(roles: any, fun?: any) {
      let accessedRoutes
      const rolesArr = typeof roles !== 'object' ? roles.split(',') : roles

      if (userStore.username === 'admin' || userStore.username === 'root') {
        accessedRoutes = asyncRoutes[0].children
      }
      else {
        accessedRoutes = await filterAsyncRoutes(menu, rolesArr)
      }

      dynamicRoutes.value = cloneDeep(accessedRoutes as any)
      if (fun) {
        fun()
      }
    }

    function handleRoutes(route: any) {
      routes.value = route
    }
    return {
      routes,
      dynamicRoutes,
      setRoutes,
      hasPermission,
      filterAsyncRoutes,
    }
  },
)

export default usePermissionStore
