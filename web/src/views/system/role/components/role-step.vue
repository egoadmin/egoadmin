<script lang="ts" setup>
import { t } from '@/i18n'

const props = defineProps({
  // 当前选择步骤
  step: {
    type: Number,
    default: 1,
  },
  // 步骤条渲染数组
  list: {
    type: Array,
    default: () => [] as string[],
  },
  // 已选择过的步骤
  selectedStep: {
    type: Array,
    default: () => [] as string[],
  },
  // 是否显示
  show: {
    type: Boolean,
    default: true,
  },
})
const emit = defineEmits(['stepChange'])
const stepCom = computed(() => {
  return props.step
})
const list = computed(() => props.list as string[])
const selectedStep = computed(() => props.selectedStep as string[])
const show = computed(() => props.show)

// 步骤切换
function stepSwitch(index: number) {
  if (index <= selectedStep.value.length) {
    emit('stepChange', index)
  }
}

// 获取bar1样式宽度
function selectedStepSome(row: string) {
  // 判断是否是已选择步骤
  return selectedStep.value.includes(row)
}

// 获取步骤信息各状态时样式类步骤(选择、未选择、已选择)
function getBaseBoxClass(row: string, index: number) {
  const str = 'base-box'
  return stepCom.value === index
    ? `${str} stepSelection`
    : `${str} ${selectedStepSome(row) ? 'selected' : 'unselected'}`
}

// 判断未选择和已选择line长度
function getBarWidth(index: number) {
  if (stepCom.value === index) {
    return selectedStepSome(list.value[index]) ? 'bar-1 selected-bar' : 'bar-1'
  }
  return 'bar-1 selected-bar'
}
</script>

<template>
  <div
    id="step-bar"
    class="step-bar"
    :style="{ display: show ? 'flex' : 'none' }"
  >
    <div v-for="(item, index) in list" :key="index">
      <div
        :class="getBaseBoxClass(item, index + 1)"
        @click="stepSwitch(index + 1)"
      >
        <div v-if="selectedStepSome(item) && step !== index + 1">
          <al-icon name="#icon-check-circle" class="icon" />
        </div>
        <div v-else class="step-one-icon">
          {{ index + 1 }}
        </div>
        <div class="base-info">
        {{ t(item) }}
        </div>
      </div>

      <div v-if="index !== list.length - 1" class="step-line">
        <div v-show="selectedStepSome(item)" class="step-one">
          <span :class="getBarWidth(index + 1)" />
          <span class="bar-2" />
          <span class="bar-3" />
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.icon {
  color: var(--el-color-primary);
  font-size: 36px;
}

.step-bar {
  width: calc(100% - 27px - 48px);
  height: 62px;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  font-family: "PingFang SC-Medium", "PingFang SC";
  font-weight: 500;

  & > div {
    display: flex;
    align-items: center;
  }

  .base-box {
    display: flex;
    align-items: center;
    position: relative;
    cursor: default;
  }

  .selected {
    cursor: pointer;

    .step-one-icon {
      background: var(--surface);
      color: var(--el-color-primary);
      border: 1px solid var(--el-color-primary);
    }
  }

  .unselected {
    .step-one-icon {
      background: var(--surface-3);
      color: var(--faint);
    }

    .base-info {
      color: var(--faint);
    }
  }

  .stepSelection {
    cursor: pointer;

    .base-info {
      color: var(--el-color-primary);
    }
  }

  .step-one-icon {
    width: 32px;
    height: 32px;
    background: var(--el-color-primary);
    border-radius: 50%;
    color: white;
    line-height: 32px;
    text-align: center;
    font-size: 16px;
    margin: 2px;
  }

  .base-info {
    font-size: 16px;
    color: var(--fg);
    margin-left: 12px;
  }

  .step-line {
    width: 144px;
    background-color: var(--surface-3);
    border-radius: 2px;
    height: 2px;
    display: flex;
    margin: 0 16px;
    overflow: hidden;

    .step-one {
      height: 100%;
      display: flex;

      .bar-1 {
        width: 50px;
        height: 100%;
        background: var(--el-color-primary);
        border-radius: 2px;
        margin-right: 2px;
      }

      .selected-bar {
        width: 1000px;
      }

      .bar-2 {
        width: 6px;
        height: 100%;
        background: var(--el-color-primary);
        border-radius: 2px;
        margin-right: 2px;
      }

      .bar-3 {
        width: 4px;
        height: 100%;
        background: var(--el-color-primary);
        border-radius: 2px;
      }
    }

    .step-two {
      height: 100%;
      display: flex;
      background-color: var(--el-color-primary);
      width: 100%;
    }
  }

  .step-two-icon {
    margin-right: 12px;
    width: 32px;
    height: 32px;
    background: var(--surface-3);
    border-radius: 50%;
    text-align: center;
    line-height: 32px;
    color: var(--faint);
    font-size: 16px;
  }

  .at-2 {
    background: var(--el-color-primary);
    color: white;
  }

  .create-vm {
    font-size: 16px;
    color: var(--faint);
  }

  .at-2-text {
    font-size: 16px;
    color: var(--fg);
  }
}
</style>
