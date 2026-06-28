<script lang="ts" setup>
import { t } from '@/i18n'

// =====删除与提示弹框======

const props = defineProps({
  visible: {
    type: Boolean,
    default: false,
  },
  // 文本内容
  content: {
    type: String,
    default: '',
  },
  // 标题
  title: {
    type: String,
    default: '',
  },
  // 宽
  width: {
    type: String,
    default: '400px',
  },
  // 关闭事件
  close: {
    type: Function,
    default: () => {},
  },
  // 取消事件
  cancel: {
    type: Function,
    default: () => {},
  },
  // 确认事件
  confirm: {
    type: Function,
    default: () => {},
  },
  // 取消是否显示
  cancelIF: {
    type: Boolean,
    default: true,
  },
  // 确认是否显示
  confirmIF: {
    type: Boolean,
    default: true,
  },
  // 'icon-error',
  // 'icon-success',
  // 'icon-warning',
  iconClassName: {
    type: String,
    default: 'icon-error',
  },
  // 内容文字size
  fontSize: {
    type: String,
    default: '18px',
  },
})

const dialogVisible = ref(false)

const title = computed(() => props.title || t('app.tip'))
const fontSize = computed(() => props.fontSize)
const { close, cancelIF, cancel, confirm, confirmIF }: any = props

function cancelChange() {
  cancel()
}
function confirmChange() {
  confirm()
}
function dialogVisibleChange(val: boolean) {
  dialogVisible.value = val
}
defineExpose({
  dialogVisibleChange,
})
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    :close-on-click-modal="false" align-center :width="width" :title="title"
    @close="close"
  >
    <div
      class="flex"
      :style="{ fontSize }"
    >
      <al-icon name="#icon-ziyuan-4" :class="`${iconClassName} icon`" />
      <div style="align-self: center;">
        {{ content }}
      </div>
    </div>

    <template #footer>
      <span class="dialog-footer">
        <el-button v-if="cancelIF" v-blur bg text @click="cancelChange">{{ t('common.cancel') }}</el-button>
        <el-button v-if="confirmIF" v-blur type="primary" @click="confirmChange">
          {{ t('common.confirm') }}
        </el-button>
      </span>
    </template>
  </el-dialog>
</template>

<style lang="scss" scoped>
.flex {
  color: var(--fg);
  font-weight: 500;
  display: flex;
  // align-items: center;
}

.icon {
  font-size: 32px;
  margin-right: 8px;
  flex-shrink: 0;
}
</style>
