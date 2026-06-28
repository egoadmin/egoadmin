<script setup lang="ts">
import eventBus from '@/utils/eventBus'
import useSettingsStore from '@/store/modules/settings'

const isShow = ref(false)

const settingsStore = useSettingsStore()

onMounted(() => {
  eventBus.on('global-hotkeys-intro-toggle', () => {
    isShow.value = !isShow.value
  })
})
</script>

<template>
  <div>
    <el-drawer v-model="isShow" :title="$t('layout.hotkeysTitle')" direction="rtl" :size="360">
      <el-descriptions :title="$t('layout.hotkeysGlobal')" :column="1" border>
        <el-descriptions-item :label="$t('layout.hotkeysViewSystemInfo')">
          {{ settingsStore.os === 'mac' ? '⌥' : 'Alt' }} + I
        </el-descriptions-item>
        <el-descriptions-item v-if="settingsStore.settings.navSearch.enable && settingsStore.settings.navSearch.enableHotkeys" :label="$t('layout.hotkeysOpenSearch')">
          {{ settingsStore.os === 'mac' ? '⌥' : 'Alt' }} + S
        </el-descriptions-item>
      </el-descriptions>
      <el-descriptions v-if="settingsStore.settings.menu.enableHotkeys && ['side', 'head'].includes(settingsStore.settings.menu.menuMode)" :title="$t('layout.hotkeysMainNav')" :column="1" border>
        <el-descriptions-item :label="$t('layout.hotkeysNextMainNav')">
          {{ settingsStore.os === 'mac' ? '⌥' : 'Alt' }} + `
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<style lang="scss" scoped>
:deep(.el-drawer__header) {
  margin-bottom: initial;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--el-border-color);
  transition: var(--el-transition-border);
}

:deep(.el-descriptions) {
  margin-bottom: 20px;

  .el-descriptions__label {
    width: 200px;
  }
}
</style>
