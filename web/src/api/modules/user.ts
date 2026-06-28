import api from '../index'

export default {
  // 获取登录加密参数
  getLoginCrypto: (data: { username: string; ua: string; action?: string }) =>
    api.post('/user.v1.UserService/GetLoginCrypto', data),

  // 登录
  login: (data: {
    username?: string
    ua?: string
    token?: string
    passwordCipher?: string
    keyId?: string
    challengeId?: string
  }) => api.post('/user.v1.UserService/Login', data),

  // 获取用户资料包含权限
  getCenterInfo: () => api.post('/user.v1.CenterService/GetCenterInfo'),

  // 修改密码
  passwordEdit: (data: { passwordCipher: string; keyId: string; challengeId: string }) =>
    api.post('/user.v1.CenterService/EditCenterPassword', data),

  // 修改头像
  editCenterAvatar: (data: { referenceId: string }) =>
    api.post('/user.v1.CenterService/EditCenterAvatar', data),

  // 修改个人资料
  editCenterInfo: (data: {
    name?: string
    gender?: number
    phone?: string
    passwordCipher?: string
    keyId?: string
    challengeId?: string
  }) => api.post('/user.v1.CenterService/EditCenterInfo', data),

  // 用户管理==========================
  // 获取用户列表
  getUserList: (data: any) => api.post('/user.v1.UserService/GetUserList', data),

  // 查询用户
  getUser: (data: any) => api.post('/user.v1.UserService/GetUser', data),

  // 删除用户
  deleteUser: (data: any) => api.post('/user.v1.UserService/DeleteUser', data),

  // 修改用户
  updateUser: (data: any) => api.post('/user.v1.UserService/UpdateUser', data),

  // 新增用户
  addUser: (data: any) => api.post('/user.v1.UserService/AddUser', data),

  // 重置密码
  resetUserPassword: (data: any) => api.post('/user.v1.UserService/ResetUserPassword', data),

  // 在线用户列表
  getOnlineUserList: (data: any) => api.post('/user.v1.UserService/GetOnlineUserList', data),

  // 下线用户
  offlineUser: (data: any) => api.post('/user.v1.UserService/OfflineUser', data),

  // 获取系统日志列表
  getLogList: (data: any) => api.post('/user.v1.LogService/GetLogList', data),

  // 用户心跳
  heartBeatUser: () => api.post('/user.v1.UserService/HeartBeatUser'),

  // 退出登录
  logout: () => api.post('/user.v1.UserService/Logout'),

  // 获取菜单
  getMenus: () => api.post('/user.v1.UserService/GetMenus'),
}
