import { viteMockServe } from 'vite-plugin-mock'

export default function createMock(env: Record<string, string>, isBuild: boolean) {
  const { VITE_BUILD_MOCK } = env
  return viteMockServe({
    mockPath: 'src/mock',
    enable: !isBuild || VITE_BUILD_MOCK === 'true',
  })
}
