import type { PluginOption } from 'vite-plus'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'
import vueLegacy from '@vitejs/plugin-legacy'

import createInspector from './inspector.ts'
import createAutoImport from './auto-import.ts'
import createComponents from './components.ts'
import createSetupExtend from './setup-extend.ts'
import createSvgIcon from './svg-icon.ts'
import createMock from './mock.ts'
import createLayouts from './layouts.ts'
import createPages from './pages.ts'
import createCompression from './compression.ts'
import createSpritesmith from './spritesmith.ts'
import createDropConsole from './drop-console.ts'

export default function createVitePlugins(viteEnv: Record<string, string>, isBuild = false) {
  const vitePlugins: (PluginOption | PluginOption[])[] = [
    vue(),
    vueJsx(),
    vueLegacy({
      renderLegacyChunks: false,
      modernPolyfills: ['es.array.at', 'es.array.find-last'],
    }),
  ]
  vitePlugins.push(createInspector())
  vitePlugins.push(createAutoImport())
  vitePlugins.push(createComponents())
  vitePlugins.push(createSetupExtend())
  vitePlugins.push(createSvgIcon(isBuild))
  vitePlugins.push(createMock(viteEnv, isBuild))
  vitePlugins.push(createLayouts())
  vitePlugins.push(createPages())
  if (isBuild) {
    vitePlugins.push(...createCompression(viteEnv))
    vitePlugins.push(createDropConsole())
  }
  vitePlugins.push(...createSpritesmith(isBuild))
  return vitePlugins
}
