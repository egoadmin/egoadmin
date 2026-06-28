import removeConsole from 'vite-plugin-remove-console'
import type { Plugin } from 'vite-plus'

// 在生产构建中移除 console.* 语句，兼容 vite-plus/Rolldown。
export default function createDropConsole(): Plugin {
  return removeConsole() as Plugin
}
