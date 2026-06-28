/// <reference types="vite-plus/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, unknown>
  export default component
}

declare module 'fingerprintjs2'

declare module 'vite-plugin-spritesmith'

declare module 'virtual:svg-icons-register'
