<script lang="ts" setup name="Pagination">
import useSettingsStore from '@/store/modules/settings'

const props = defineProps({
  /**
   * currentPage: 1,
  pageSize: 10, // 一页的大小
  total: 16,
  pageSizes: [10, 20, 30, 40, 50],
  */
  pageState: {
    type: Object,
    required: true,
  },
  // 同el-pagination
  layout: {
    type: String,
    required: true,
  },
  // 每一行的高度
  rowHeight: {
    type: Number,
    required: true,
  },
  // 用于计算表格高度的需要减去的固定高度(要包含表头高度)
  subHeight: {
    type: Number,
    required: true,
  },
  // 表中的数据条数
  dataLength: {
    type: Number,
    required: true,
  },
  marginLeft: {
    type: Number,
    default: 16,
  },
  marginRight: {
    type: Number,
    default: 16,
  },
})

const emits = defineEmits(['sizeChange', 'currentChange'])
const paginationRef = ref()
const currentPage = ref(1)
const paginationHeight = 32
const paginationMarginTop = 20
const paginationState = reactive({
  currentPage: 1,
  pageSize: 10, // 一页的大小
  total: 16,
  pageSizes: [10, 20, 30, 40, 50],
})

const pageStateCom = computed(() => {
  return props.pageState
})
const subHeightCom = computed(() => {
  return props.subHeight
})
const rowHeightCom = computed(() => {
  return props.rowHeight
})
const dataLengthCom = computed(() => {
  return props.dataLength
})
const settingsStore = useSettingsStore()
// 侧边栏次导航当前实际宽度
const subSidebarActualWidth = computed(() => {
  let actualWidth = parseInt(getComputedStyle(document.documentElement).getPropertyValue('--g-sub-sidebar-width'))
  if (settingsStore.settings.menu.subMenuCollapse) {
    actualWidth = 64
  }
  if (settingsStore.isTabbarHorizontal) {
    actualWidth = 0
  }
  return actualWidth
})
function updatePagination() {
  const subHeight = subHeightCom.value || 0
  const rowHeight = rowHeightCom.value || 0
  const dataLength = dataLengthCom.value || 0

  if (rowHeight * dataLength > window.innerHeight - subHeight - paginationHeight - paginationMarginTop) {
    console.log(true)

    paginationRef.value.style.position = 'fixed'
    paginationRef.value.style.bottom = '0'
    paginationRef.value.style.zIndex = 10
    paginationRef.value.style.borderTop = '1px solid var(--border)'
    paginationRef.value.style.marginTop = '0'
    paginationRef.value.style.right = '0'

    if (!settingsStore.isTabbarHorizontal) {
      // 左右布局
      const pageMain: any = document.querySelector('#pageMain')
      console.log(getComputedStyle(pageMain).width)
      paginationRef.value.style.width = getComputedStyle(pageMain).width
      paginationRef.value.style.right = '16px'
    }
    else {
      // 上下布局
      paginationRef.value.style.right = '0'
      paginationRef.value.style.width = '100%'
    }
  }
  else {
    console.log(false)
    paginationRef.value.style.position = 'static'
    paginationRef.value.style.zIndex = 0
    paginationRef.value.style.width = '100%'
    paginationRef.value.style.borderTop = 'none'
    paginationRef.value.style.marginTop = `${paginationMarginTop}px`
  }
}
function onResize() {
  if (!paginationRef.value) {
    return
  }
  updatePagination()
}
function onSizeChange(value: number) {
  pageStateCom.value.pageSize = value
  emits('sizeChange', value)
}
function onCurrentChange(value: number) {
  pageStateCom.value.currentPage = value
  emits('currentChange', value)
}
watch(() => currentPage.value, (value: any, old: any) => {
  pageStateCom.value.currentPage = value
}, {
  deep: true,
})

watch(() => pageStateCom.value, (value: any, old: any) => {
  if (value) {
    paginationState.currentPage = value.currentPage
    paginationState.pageSize = value.pageSize
    paginationState.total = value.total
    paginationState.pageSizes = value.pageSizes
    currentPage.value = value.currentPage
  }
}, {
  deep: true,
  immediate: true,
})

// const total = computed(()=>{
//   pageState.total <= pageState.pageSize[0];
//   props.layout.
// })

onMounted(() => {
  const pagination: any = document.querySelector('#pagination')
  const resizeObserver = new ResizeObserver(() => {
    onResize()
  })
  window.addEventListener('resize', () => {
    onResize()
  })
  resizeObserver.observe(pagination?.parentNode?.parentNode)
})
</script>

<template>
  <div id="pagination" ref="paginationRef">
    <el-pagination
      v-model:current-page="currentPage" :page-sizes="pageState.pageSizes"
      :page-size="pageState.pageSize" :layout="props.layout" :total="pageState.total"
      @size-change="onSizeChange" @current-change="onCurrentChange"
    />
  </div>
</template>

<style lang="scss" scoped>
#pagination {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  background-color: var(--surface);
  padding: 8px;
}
</style>
