import { createProdMockServer } from 'vite-plugin-mock/client'

const mocks: any[] = []
const mockContext = import.meta.glob('./mock/*.ts', { eager: true })
Object.keys(mockContext).forEach((v) => {
  mocks.push(...(mockContext[v] as any).default)
})

export function setupProdMockServer() {
  void createProdMockServer(mocks)
}
