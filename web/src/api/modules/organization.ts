import api from '../index'

export default {
  // 新增组织
  addDept: (data: any) => api.post('/user.v1.DeptService/AddDept', data),

  // 检查组织是否可删除
  checkDeptDelete: (data: any) => api.post('/user.v1.DeptService/CheckDeptDelete', data),

  //  级联删除组织(含子组织)
  deleteDeptCascade: (data: any) => api.post('/user.v1.DeptService/DeleteDeptCascade', data),

  // 查询组织
  getDept: (data: any) => api.post('/user.v1.DeptService/GetDept', data),

  // 根据组织名称获取组织
  getDeptByName: (data: any) => api.post('/user.v1.DeptService/GetDeptByName', data),

  // 获取子组织
  getDeptChild: (data: any) => api.post('/user.v1.DeptService/GetDeptChild', data),

  // 获取顶级组织
  getDeptTop: () => api.post('/user.v1.DeptService/GetDeptTop'),

  // 修改组织
  updateDept: (data: any) => api.post('/user.v1.DeptService/UpdateDept', data),

  // 修改排序
  updatePriorityDept: (data: any) => api.post('/user.v1.DeptService/UpdatePriorityDept', data),

  // 根据组织名称获取组织用户
  uGetDept: (data: any) => api.post('/user.v1.UserService/UGetDept', data),
}
