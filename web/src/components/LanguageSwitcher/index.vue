<script lang="ts" setup name="LanguageSwitcher">
import { ElMessage } from 'element-plus'
import { localeOptions, localeRef, setLocale, t, type LocaleKey } from '@/i18n'

const currentLocale = computed<LocaleKey>(() => localeRef.value as LocaleKey)

const currentLabel = computed(() => {
  return localeOptions.find(item => item.value === currentLocale.value)?.label ?? ''
})

// 顶栏短标签（设计图：中 / EN）
const shortLabel = computed(() => (currentLocale.value === 'zh-CN' ? '中' : 'EN'))

function switchLocale(command: string | number | object) {
  const locale = command as LocaleKey
  if (locale === currentLocale.value) {
    return
  }
  setLocale(locale)
  ElMessage.success(t('app.languageChanged'))
}
</script>

<template>
  <el-dropdown trigger="click" @command="switchLocale">
    <span class="language-trigger item" :title="t('app.currentLanguage')">
      <span class="label">{{ shortLabel }}</span>
    </span>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          v-for="item in localeOptions"
          :key="item.value"
          :command="item.value"
          :disabled="item.value === currentLocale"
        >
          {{ item.label }}
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<style lang="scss" scoped>
.language-trigger {
  display: inline-flex;
  align-items: center;
  height: 34px;
  padding: 0 10px;
  outline: none;
  cursor: pointer;
  border-radius: var(--r-md);
  transition: background-color 0.15s var(--ease);

  &:hover {
    background-color: var(--surface-2);
  }

  .label {
    font-size: 13px;
    color: var(--muted);
  }

  &:hover .label {
    color: var(--fg);
  }
}
</style>
