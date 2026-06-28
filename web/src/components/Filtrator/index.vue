<script lang="ts" setup>
import { t } from '@/i18n'

const props = defineProps({
  checkedValue: {
    type: Array,
    default: () => [],
  },
  // 触发器样式：icon（漏斗图标）| button（带「筛选」文字的按钮，对齐设计图）
  triggerType: {
    type: String,
    default: 'icon',
  },
})
const checkedValue = computed(() => props.checkedValue.some((item: any) => {
  if (typeof item === 'object' && item) {
    const obj = Object.keys(item)
    return !!obj.length
  }
  return !!item
}))
const visible = ref(false)
function visibleChange() {
  visible.value = !visible.value
}
</script>

<template>
  <div v-if="visible" class="bg" @click="() => visible = false" />
  <div class="box" @click.stop>
    <button
      v-if="triggerType === 'button'"
      class="filter-btn"
      :class="{ act: checkedValue || visible }"
      @click="visibleChange"
    >
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
        <path d="M3 5h18l-7 8v6l-4 2v-8z" stroke-linejoin="round" />
      </svg>
      {{ t('common.filter') }}
    </button>
    <el-tooltip
      v-else
      effect="dark"
      :content="t('common.filter')"
      placement="top"
    >
      <div :class="{ active: visible } " class="filter" @click="visibleChange">
        <al-icon v-if="checkedValue" name="#icon-a-zu45667" class="checkSvg" />
        <al-icon v-else name="#icon-filter1" />
      </div>
    </el-tooltip>
    <div v-if="visible" class="area">
      <slot />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.bg {
  width: 100vw;
  height: 100vh;
  position: fixed;
  top: 0;
  left: 0;
  z-index: 990;
}

.box {
  position: relative;
  display: inline-flex;
}

.filter {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border-radius: var(--r-md);

  svg {
    color: var(--muted);
  }

  &:hover svg {
    color: var(--accent);
  }
}

.active {
  background-color: var(--surface-2);
}

// 「筛选」按钮触发器（设计图 .filter-btn）
.filter-btn {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  height: 36px;
  padding: 8px 14px;
  font: inherit;
  font-size: 14px;
  font-weight: 550;
  color: var(--text);
  cursor: pointer;
  background: var(--surface);
  border: 1px solid var(--border-strong);
  border-radius: var(--r-md);
  transition: border-color 0.18s var(--ease), color 0.18s var(--ease);

  &:hover {
    color: var(--accent-text);
    border-color: var(--accent);
  }

  &.act {
    color: var(--accent-text);
    border-color: var(--accent);
  }
}

.checkSvg {
  color: var(--accent) !important;
}

.area {
  position: absolute;
  top: 42px;
  right: -10px;
  background: var(--surface);
  border: 1px solid var(--border);
  box-shadow: var(--shadow-md);
  border-radius: var(--r-lg);
  padding: 16px 16px 24px;
  z-index: 1000;
}
</style>
