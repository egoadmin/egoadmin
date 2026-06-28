import 'vue'

declare module 'vue' {
  interface ComponentCustomProperties {
    $toCustomDate: (value: string, format?: string) => string
  }
}
