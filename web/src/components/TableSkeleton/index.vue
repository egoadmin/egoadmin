<script lang="ts" setup name="TableSkeleton">
// 表格骨架屏（设计 states.html）：保留表头，表体用等高占位行 + shimmer 微光
const props = defineProps({
  // 骨架行数
  rows: {
    type: Number,
    default: 6,
  },
  // 每列灰条基准宽度（最后一列右对齐）
  columns: {
    type: Array as () => string[],
    default: () => ['22%', '40%', '14%', '18%'],
  },
})

// 为每行生成轻微宽度抖动，避免完全整齐的呆板感
const rowList = computed(() => {
  const deltas = [0, -2, 2, -4, 0, 2, -2, 4]
  return Array.from({ length: props.rows }, (_, r) =>
    props.columns.map((w, c) => {
      const num = Number.parseFloat(w)
      if (Number.isNaN(num)) {
        return w
      }
      const d = deltas[(r + c) % deltas.length]
      return `${Math.max(8, num + d)}%`
    }),
  )
})
</script>

<template>
  <div class="sk-body">
    <div v-for="(row, r) in rowList" :key="r" class="sk-row">
      <div
        v-for="(w, c) in row"
        :key="c"
        class="sk"
        :class="{ 'sk-last': c === row.length - 1 }"
        :style="{ width: w }"
      />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.sk-body {
  width: 100%;
}

.sk-row {
  display: flex;
  align-items: center;
  gap: 18px;
  height: 56px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border);

  &:last-child {
    border-bottom: 0;
  }
}

.sk {
  position: relative;
  height: 12px;
  overflow: hidden;
  background: var(--surface-2);
  border-radius: 6px;

  &.sk-last {
    margin-left: auto;
  }

  &::after {
    content: '';
    position: absolute;
    inset: 0;
    transform: translateX(-100%);
    background: linear-gradient(
      90deg,
      transparent,
      color-mix(in oklab, var(--surface) 75%, transparent),
      transparent
    );
    animation: sk-shimmer 1.3s infinite;
  }
}

@keyframes sk-shimmer {
  100% {
    transform: translateX(100%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .sk::after {
    animation: none;
  }
}
</style>
