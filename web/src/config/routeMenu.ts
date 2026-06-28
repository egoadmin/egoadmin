import { APIs } from '../api/api-manifest'

interface Menu {
  title: string
  id: number
  child?: Array<any>
  [key: string]: any
}

export const menu: Menu[] = [
  {
    title: 'app.home',
    id: 1,
    path: '/',
    disabled: true,
  },
  {
    title: 'menu.systemSettings',
    id: 2,
    path: '/system',
    child: [
      {
        title: 'menu.roleManagement',
        id: 201,
        path: 'role',
        child: [
          {
            title: 'role.init',
            id: 20101,
            apiList: [
              APIs.user.v1.RoleService.GetRoleList,
              APIs.user.v1.RoleService.GetRoleAll,
              APIs.user.v1.UserService.GetAPIs,
            ].join(','),
            path: 'role',
          },
          {
            title: 'common.add',
            id: 20102,
            apiList: APIs.user.v1.RoleService.AddRole,
          },
          {
            title: 'common.edit',
            id: 20103,
            apiList: [APIs.user.v1.RoleService.GetRole, APIs.user.v1.RoleService.UpdateRole].join(
              ',',
            ),
          },
          {
            title: 'common.delete',
            id: 20104,
            apiList: [
              APIs.user.v1.RoleService.DeleteRole,
              APIs.user.v1.RoleService.CheckDeleteRole,
            ].join(','),
          },
          {
            title: 'common.detail',
            id: 20105,
            apiList: APIs.user.v1.RoleService.GetRole,
          },
        ],
      },
      {
        title: 'menu.onlineUsers',
        id: 202,
        path: 'online_user',
        child: [
          {
            title: 'role.init',
            id: 20201,
            apiList: APIs.user.v1.UserService.GetOnlineUserList,
            path: 'online_user',
          },
          {
            title: 'role.forceLogout',
            id: 20202,
            apiList: APIs.user.v1.UserService.OfflineUser,
          },
        ],
      },
      {
        title: 'menu.operationLogs',
        id: 203,
        path: 'operation_log',
        child: [
          {
            title: 'role.init',
            id: 20301,
            apiList: APIs.user.v1.LogService.GetLogList,
            path: 'operation_log',
          },
        ],
      },
    ],
  },
  {
    title: 'menu.userManagement',
    id: 3,
    path: '/user_management',
    child: [
      {
        title: 'menu.organizationManagement',
        id: 301,
        path: 'organization',
        child: [
          {
            title: 'role.init',
            id: 30101,
            apiList: [
              APIs.user.v1.DeptService.GetDeptTop,
              APIs.user.v1.DeptService.GetDeptChild,
              APIs.user.v1.DeptService.GetDeptByName,
              APIs.user.v1.DeptService.UpdatePriorityDept,
            ].join(','),
            path: 'organization',
          },
          {
            title: 'common.add',
            id: 30102,
            apiList: APIs.user.v1.DeptService.AddDept,
          },
          {
            title: 'common.edit',
            id: 30103,
            apiList: APIs.user.v1.DeptService.UpdateDept,
          },
          {
            title: 'common.delete',
            id: 30104,
            apiList: [
              APIs.user.v1.DeptService.CheckDeptDelete,
              APIs.user.v1.DeptService.DeleteDeptCascade,
            ].join(','),
          },
        ],
      },
      {
        title: 'menu.userList',
        id: 302,
        path: 'user',
        child: [
          {
            title: 'role.init',
            id: 30201,
            apiList: [
              APIs.user.v1.UserService.GetUserList,
              APIs.user.v1.UserService.UGetDept,
              APIs.user.v1.RoleService.GetRoleAll,
            ].join(','),
            path: 'user',
          },
          {
            title: 'common.add',
            id: 30202,
            apiList: APIs.user.v1.UserService.AddUser,
          },
          {
            title: 'common.edit',
            id: 30203,
            apiList: [APIs.user.v1.UserService.UpdateUser, APIs.user.v1.UserService.GetUser].join(
              ',',
            ),
          },
          {
            title: 'common.delete',
            id: 30204,
            apiList: APIs.user.v1.UserService.DeleteUser,
          },
          {
            title: 'role.resetPassword',
            id: 30205,
            apiList: APIs.user.v1.UserService.ResetUserPassword,
          },
          // {
          //   title: '启用/禁用',
          //   id: 30206,
          //   apiList: '',
          // },
        ],
      },
    ],
  },
]
