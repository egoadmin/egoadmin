<script lang="ts" setup>
import eruda from 'eruda'
import VConsole from 'vconsole'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import hotkeys from 'hotkeys-js'
import eventBus from './utils/eventBus'
import useSettingsStore from '@/store/modules/settings'
import useUserStore from '@/store/modules/user'
import { localeRef, t } from '@/i18n'

const userStore = useUserStore()
const settingsStore = useSettingsStore()
const { auth } = useAuth()

const buttonConfig = ref({
  autoInsertSpace: true,
})
const elementLocale = computed(() => (localeRef.value === 'en' ? en : zhCn))

// 登录获取详情与心跳
if (userStore.token) {
  void userStore.bootstrapSession()
}

// 侧边栏主导航当前实际宽度
const mainSidebarActualWidth = computed(() => {
  let actualWidth = parseInt(
    getComputedStyle(document.documentElement).getPropertyValue('--g-main-sidebar-width'),
  )
  if (['head', 'single'].includes(settingsStore.settings.menu.menuMode)) {
    actualWidth = 0
  }
  return `${actualWidth}px`
})

// 侧边栏次导航当前实际宽度
const subSidebarActualWidth = computed(() => {
  let actualWidth = parseInt(
    getComputedStyle(document.documentElement).getPropertyValue('--g-sub-sidebar-width'),
  )
  if (settingsStore.settings.menu.subMenuCollapse) {
    actualWidth = 64
  }
  if (settingsStore.isTabbarHorizontal) {
    actualWidth = 0
  }
  return `${actualWidth}px`
})

watch(
  [() => settingsStore.settings.app.enableDynamicTitle, () => settingsStore.title, () => localeRef.value],
  () => {
    if (settingsStore.settings.app.enableDynamicTitle && settingsStore.title) {
      const title = typeof settingsStore.title === 'function'
        ? settingsStore.title()
        : t(settingsStore.title)
      document.title = `${title} - ${import.meta.env.VITE_APP_TITLE}`
    } else {
      document.title = import.meta.env.VITE_APP_TITLE
    }
  },
  {
    immediate: true,
  },
)

onMounted(() => {
  settingsStore.setMode(document.documentElement.clientWidth)
  window.onresize = () => {
    settingsStore.setMode(document.documentElement.clientWidth)
  }
  hotkeys('alt+i', () => {
    eventBus.emit('global-system-info-toggle')
  })
})

if (import.meta.env.VITE_APP_DEBUG_TOOL === 'eruda') {
  eruda.init()
}
if (import.meta.env.VITE_APP_DEBUG_TOOL === 'vconsole') {
  new VConsole()
}
</script>

<template>
  <el-config-provider
    :locale="elementLocale"
    :size="settingsStore.settings.app.elementSize"
    :button="buttonConfig"
  >
    <RouterView
      v-slot="{ Component, route }"
      :style="{
        '--g-main-sidebar-actual-width': mainSidebarActualWidth,
        '--g-sub-sidebar-actual-width': subSidebarActualWidth,
      }"
    >
      <component :is="Component" v-if="auth(route.meta.auth ?? '')" />
      <not-allowed v-else />
    </RouterView>
    <system-info />
  </el-config-provider>
</template>
