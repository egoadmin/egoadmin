<script setup lang="ts">
import eventBus from '@/utils/eventBus'
import { t } from '@/i18n'

const isShow = ref(false)

const { pkg, lastBuildTime } = __SYSTEM_INFO__

onMounted(() => {
  eventBus.on('global-system-info-toggle', () => {
    isShow.value = !isShow.value
  })
})
</script>

<template>
  <div>
    <el-drawer v-model="isShow" :title="t('systemInfo.title')" direction="rtl" :size="360">
      <el-descriptions direction="vertical" :column="1" border>
        <el-descriptions-item :label="t('systemInfo.version')" align="center">
          {{ pkg.version }}
        </el-descriptions-item>
        <el-descriptions-item :label="t('systemInfo.lastBuildTime')" align="center">
          {{ lastBuildTime }}
        </el-descriptions-item>
      </el-descriptions>
      <el-descriptions :title="t('systemInfo.dependencies')" :column="1" size="small" border>
        <el-descriptions-item v-for="(val, key) in (pkg.dependencies as object)" :key="key" :label="key">
          {{ val }}
        </el-descriptions-item>
      </el-descriptions>
      <el-descriptions :title="t('systemInfo.devDependencies')" :column="1" size="small" border>
        <el-descriptions-item v-for="(val, key) in (pkg.devDependencies as object)" :key="key" :label="key">
          {{ val }}
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
}
</style>
