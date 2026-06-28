<script lang="ts" setup name="DefaultPage">
import { t } from '@/i18n'

const props = defineProps({
  // 无数据
  noData: {
    type: Boolean,
    default: false,
  },
  // 无权限
  noPermission: {
    type: Boolean,
    default: false,
  },
  // 无搜索结果
  noSearchResults: {
    type: Boolean,
    default: false,
  },
  // 加载失败
  error: {
    type: Boolean,
    default: false,
  },
  // 无数据态主按钮文案（默认 empty.dataAction）
  addText: {
    type: String,
    default: '',
  },
  // 是否显示无数据态的主操作按钮
  showAdd: {
    type: Boolean,
    default: true,
  },
  // 无数据态自定义标题/说明（不同页面文案不同）
  dataTitle: {
    type: String,
    default: '',
  },
  dataDesc: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['add', 'reset', 'retry'])

// 当前状态：优先级 error > noPermission > noSearchResults > noData
type StateKey = 'error' | 'permission' | 'search' | 'data'
const current = computed<StateKey>(() => {
  if (props.error) {
    return 'error'
  }
  if (props.noPermission) {
    return 'permission'
  }
  if (props.noSearchResults) {
    return 'search'
  }
  return 'data'
})

const config = computed(() => {
  switch (current.value) {
    case 'error':
      return { icon: 'err', danger: true, title: t('empty.errorTitle'), desc: t('empty.errorDesc') }
    case 'permission':
      return { icon: 'lock', danger: false, title: t('empty.permissionTitle'), desc: t('empty.permissionDesc') }
    case 'search':
      return { icon: 'search', danger: false, title: t('empty.searchTitle'), desc: t('empty.searchDesc') }
    default:
      return {
        icon: 'box',
        danger: false,
        title: props.dataTitle || t('empty.dataTitle'),
        desc: props.dataDesc || t('empty.dataDesc'),
      }
  }
})

const addLabel = computed(() => props.addText || t('empty.dataAction'))
</script>

<template>
  <div class="empty">
    <div class="ei" :class="{ danger: config.danger }">
      <!-- box · 无数据 -->
      <svg v-if="config.icon === 'box'" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M3 13h5l1.6 2.5h4.8L16 13h5M5 6h14l2 7v5H3v-5z" stroke-linejoin="round" />
      </svg>
      <!-- search · 无搜索结果 -->
      <svg v-else-if="config.icon === 'search'" width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
        <circle cx="11" cy="11" r="7" /><path d="M21 21l-4.3-4.3" stroke-linecap="round" />
      </svg>
      <!-- lock · 无权限 -->
      <svg v-else-if="config.icon === 'lock'" width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
        <rect x="4.5" y="10.5" width="15" height="10" rx="2" /><path d="M8 10.5V7a4 4 0 0 1 8 0v3.5" stroke-linecap="round" />
      </svg>
      <!-- err · 加载失败 -->
      <svg v-else width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
        <path d="M12 8v5m0 3h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </div>
    <h4>{{ config.title }}</h4>
    <p>{{ config.desc }}</p>

    <!-- 无数据：主按钮「新增」 -->
    <el-button v-if="current === 'data' && showAdd" v-blur type="primary" class="ea" @click="emit('add')">
      {{ addLabel }}
    </el-button>
    <!-- 无搜索结果：次按钮「清空筛选」 -->
    <el-button v-else-if="current === 'search'" v-blur class="ea" @click="emit('reset')">
      {{ t('empty.searchAction') }}
    </el-button>
    <!-- 加载失败：次按钮「重试」 -->
    <el-button v-else-if="current === 'error'" v-blur class="ea" @click="emit('retry')">
      {{ t('empty.errorAction') }}
    </el-button>
    <!-- 无权限：无按钮 -->
  </div>
</template>

<style lang="scss" scoped>
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 56px 0;
  color: var(--muted);
  line-height: 1.5;
  text-align: center;

  .ei {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 64px;
    height: 64px;
    margin-bottom: 14px;
    color: var(--faint);
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 50%;

    &.danger {
      color: var(--danger);
      background: var(--danger-weak);
      border-color: transparent;
    }
  }

  h4 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
    color: var(--fg);
  }

  p {
    max-width: 38ch;
    margin: 5px 0 0;
    font-size: 13px;
    color: var(--muted);
  }

  .ea {
    margin-top: 16px;
  }
}
</style>
