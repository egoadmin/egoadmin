import api from '../index'

export default {
  // 新增角色
  addRole: (data: any) => api.post('/user.v1.RoleService/AddRole', data),

  // 删除角色
  deleteRole: (data: any) => api.post('/user.v1.RoleService/DeleteRole', data),

  // 删除校验
  checkDeleteRole: (data: any) => api.post('/user.v1.RoleService/CheckDeleteRole', data),

  // 查询角色
  getRole: (data: any) => api.post('/user.v1.RoleService/GetRole', data),

  // 获取所有角色
  getRoleAll: () => api.post('/user.v1.RoleService/GetRoleAll'),

  // 获取角色列表
  getRoleList: (data: any) => api.post('/user.v1.RoleService/GetRoleList', data),

  // 修改角色
  updateRole: (data: any) => api.post('/user.v1.RoleService/UpdateRole', data),

  // 获取系统接口
  getAPIs: () => api.post('/user.v1.UserService/GetAPIs'),
}
