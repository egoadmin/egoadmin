import api from '../index'

export default {
  // 获取操作日志
  getOperate: (data: {
    page: number
    limit: number
  }) => api.post('system/operate', data, {
    baseURL: '/mock/',
  }),
}
