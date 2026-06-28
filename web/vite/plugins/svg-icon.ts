import path from 'node:path'
import { createSvgIconsPlugin } from 'vite-plugin-svg-icons-ng'

export default function createSvgIcon(_isBuild: boolean) {
  return createSvgIconsPlugin({
    iconDirs: [path.resolve(process.cwd(), 'src/assets/icons/')],
    symbolId: 'icon-[dir]-[name]',
  })
}
