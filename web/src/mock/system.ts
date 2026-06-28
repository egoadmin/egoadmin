export default [
  {
    url: '/mock/system/operate',
    method: 'post',
    response: ({ body }: any) => {
      if (!body.limit?.toString() || !body.page?.toString()) {
        return {
          error: '缺少分页相关信息',
          status: 2,
        }
      }
      const count = 32
      let dataPos = 1 * (body.page - 1) * body.limit + body.limit
      if (dataPos > count) {
        dataPos = count
      }
      const testData: Array<any> = []
      for (let i = 1 * (body.page - 1) * body.limit; i < dataPos; i++) {
        testData.push({
          name: `大幅度1${i}`,
          time: `5111-1-44${i}`,
          id: i,
          level: i,
          parentId: '',
        })
      }
      return {
        error: '',
        status: 1,
        data: {
          list: testData,
          count,
        },
      }
    },
  },
]
