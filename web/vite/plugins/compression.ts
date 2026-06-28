import type { Algorithm } from 'vite-plugin-compression2'
import compression from 'vite-plugin-compression2'

export default function createCompression(env: Record<string, string>) {
  const VITE_BUILD_COMPRESS = env.VITE_BUILD_COMPRESS ?? ''
  if (!VITE_BUILD_COMPRESS) {
    return []
  }
  const compressList = VITE_BUILD_COMPRESS.split(',')
  const algorithms: Algorithm[] = []
  if (compressList.includes('gzip')) {
    algorithms.push('gzip')
  }
  if (compressList.includes('brotli')) {
    algorithms.push('brotliCompress')
  }
  if (algorithms.length === 0) {
    return []
  }
  return [compression({ algorithms, deleteOriginalAssets: false })]
}
