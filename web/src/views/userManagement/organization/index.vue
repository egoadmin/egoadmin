<script lang="ts" setup>
import Sortable from 'sortablejs'
import { ElMessage } from 'element-plus'
import OrganizationOperate from './components/organization-operate.vue'
import apiOrg from '@/api/modules/organization'
import useUserStore from '@/store/modules/user'
import { t } from '@/i18n'
import { isNoPermissionMessage } from '@/utils/i18n'

const userStore = useUserStore()
const list = ref([])
const rows = ref({})
const tableRef = ref()
const activeRows: any = ref([])
const hintMsg = ref('')
const listParam = ref({
  name: '',
  key: '',
  id: '0',
})
const orgTitle = ref(t('organization.addTitle'))
const orgMode = ref<'add' | 'edit'>('add')
const currentItem = ref()
const loading = ref(true)
const tileDepts = ref([])
const expandedKeys = ref([])
const deptsData = ref([])

// 把数组平铺
function flatten(arr: any[], children = 'childs') {
  let result: any[] = []
  for (let i = 0; i < arr.length; i++) {
    result.push(arr[i])
    if (arr[i][children] && arr[i][children].length > 0) {
      result = result.concat(flatten(arr[i][children]))
    }
  }
  return result
}

// 封装一个处理函数
function toTree(list: any, parentId = '') {
  const lenNum = list.length // 获取传入参数的长度,作为for循环的循环次数
  function loop(parentId: any) { // 定义一个递归函数
    const res = [] // 定义一个空数组,作为最后返回结果的容器
    for (let i = 0; i < lenNum; i++) {
      const item = list[i]
      if (item.parentId === parentId) {
        item.children = loop(item.id)
        res.push(item)
      }
    }
    return res
  }
  return loop(parentId)
}
function getLeastIndex(index: any) {
  return Math.max(index, 1)
}

// 拖拽配置初始化
function setSort() {
  const el: any = document.querySelector('#dragTable table tbody')
  // eslint-disable-next-line no-new
  new Sortable(el, {
    group: 'dragName',
    draggable: '.el-table__row',
    handle: '.move',
    onMove({ dragged, related }: any) {
      /*
       evt.dragged; // 被拖拽的对象
       evt.related; // 被替换的对象
      */
      const oldRow: any = activeRows.value[dragged.rowIndex]
      const newRow: any = activeRows.value[related.rowIndex]
      tableRef.value.toggleRowExpansion(oldRow, false)
      console.log(oldRow, newRow, dragged.rowIndex, related.rowIndex)

      if (oldRow.parentId === newRow.parentId) {
        // 同级元素才允许拖拽
        return true
      }
      else {
        // 跨级元素禁止拖拽
        return false
      }
    },

    onEnd: async ({ oldIndex, newIndex }: any) => {
      const oldRow: any = activeRows.value[oldIndex]
      const newRow: any = activeRows.value[newIndex]

      tableRef.value.toggleRowExpansion(newRow, false)
      if (oldIndex !== newIndex && oldRow.parentId === newRow.parentId) {
        loading.value = true

        let oldRowSuffixData = activeRows.value.slice(oldIndex)
        let newRowSuffixData = activeRows.value.slice(newIndex)

        oldRowSuffixData = oldRowSuffixData.filter((d: any, i: any) => i < getLeastIndex(oldRowSuffixData.findIndex((_d: any, _i: any) => _d.parentId === oldRow.parentId && _i !== 0)))
        newRowSuffixData = newRowSuffixData.filter((d: any, i: any) => i < getLeastIndex(newRowSuffixData.findIndex((_d: any, _i: any) => _d.parentId === newRow.parentId && _i !== 0)))
        const targetRows = activeRows.value.splice(oldIndex, oldRowSuffixData.length)

        if (oldIndex > newIndex) {
          activeRows.value.splice(newIndex, 0, ...targetRows)
        }
        else if (oldIndex < newIndex) {
          activeRows.value.splice(newIndex + newRowSuffixData.length - oldRowSuffixData.length, 0, ...targetRows)
        }
        console.log(activeRows.value)
        // 排序
        const priorities: any = []
        activeRows.value.forEach((item: any) => {
          if (item.parentId === oldRow.parentId) {
            priorities.push({ id: item.id, priority: priorities.length + 1 })
          }
        })
        await apiOrg.updatePriorityDept({ priorities })
        ElMessage.success(t('organization.sortSuccess'))
        getList()
      }
    },
  })
}

const organizationOperateRef = ref()
function organizationOperateChange() {
  orgTitle.value = t('organization.addTitle')
  orgMode.value = 'add'
  currentItem.value = null
  organizationOperateRef.value.dialogOperateVisibleChange(true)
}

const hintRef = ref()
const hintDialogRef = ref()

const noSearchResults = ref(false)
const noData = ref(false)
const noPermission = ref(false)
const loadError = ref(false)
// 获取列表
async function getList() {
  loading.value = true
  noSearchResults.value = false
  noData.value = false
  noPermission.value = false
  loadError.value = false
  try {
    if (listParam.value.name.length > 50) {
      listParam.value.name = listParam.value.name.slice(0, 50)
    }
    const res: any = await apiOrg.getDeptByName(listParam.value)
    loading.value = false
    list.value = res.depts
    activeRows.value = flatten(list.value)
    // 默认展开整棵组织树（未通过组织名定位时）
    if (!listParam.value.id || listParam.value.id === '0') {
      expandedKeys.value = collectAllKeys(list.value) as any
      allExpanded.value = true
    }
    if (!listParam.value.name) {
      tileDepts.value = tileDeptsHandle(res.depts)
      deptsData.value = res.depts
    }
    if (!list.value.length) {
      noSearchResults.value = !!listParam.value.name
      noData.value = !listParam.value.name
    }
  }
  catch (err: any) {
    if (isNoPermissionMessage(err.message)) {
      noPermission.value = true
    }
    else {
      loadError.value = true
    }
    list.value = []
    loading.value = false
  }
}

// 空状态操作：清空搜索
function onResetSearch() {
  listParam.value.name = ''
  listParam.value.key = ''
  listParam.value.id = '0'
  getList()
}

// 把组织平铺添加上级组织名
function tileDeptsHandle(depts: any) {
  const arr: any = []
  const tile = (data: any, name = '', ids = []) => {
    data.forEach((item: any) => {
      arr.push({ deptName: name + item.deptName, id: item.id, level: item.level, ids: [...ids, ...[getRowKey(item)]] })
      if (item.childs) {
        tile(item.childs, `${name + item.deptName}>`, [...ids, ...[getRowKey(item)]] as any)
      }
    })
  }
  tile(depts)
  console.log(arr)

  return arr
}
// 新增组织
async function addOrg(data: any) {
  await apiOrg.addDept(data)
  organizationOperateRef.value.dialogOperateVisibleChange(false)
  getList()
  ElMessage.success(t('organization.addSuccess'))
}

// 编辑组织
async function updateOrg(data: any) {
  await apiOrg.updateDept(data)
  organizationOperateRef.value.dialogOperateVisibleChange(false)
  getList()
  ElMessage.success(t('organization.editSuccess'))
}

// 新增子组织
function addItemOpen(row: any) {
  orgTitle.value = t('organization.addTitle')
  orgMode.value = 'add'
  const { id } = row
  currentItem.value = { id, deptName: '' }
  organizationOperateRef.value.dialogOperateVisibleChange(true)
}

// 修改组织打开
async function updateOrgOpen(row: any) {
  const { id, deptName, parentId } = row
  orgTitle.value = t('organization.editTitle')
  orgMode.value = 'edit'
  currentItem.value = { id: parentId, deptName, editId: id }
  organizationOperateRef.value.dialogOperateVisibleChange(true)
}

function openDel(row: any) {
  rows.value = JSON.parse(JSON.stringify(row))
  hintRef.value.dialogVisibleChange(true)
}

// 删除组织
async function delOrg() {
  const { id }: any = rows.value
  const res: any = await apiOrg.checkDeptDelete({ id })
  if (res.isAllow) {
    await apiOrg.deleteDeptCascade({ id })
    getList()
    ElMessage.success(t('organization.deleteSuccess'))
    hintRef.value.dialogVisibleChange(false)
  }
  else {
    hintDialogRef.value.dialogVisibleChange(true)
    hintMsg.value = res.msg
  }
}

function getRowKey(row: any) {
  return row.id + row.updatedAt
}

const autocompleteRef = ref()

// 组织选择change
function deptChange(node: any, noId = false) {
  if (!noId) {
    listParam.value.id = node?.id || '0'
  }
  node?.ids ? expandedKeys.value = node.ids : expandedKeys.value = []
  autocompleteRef.value.blur()
}
function autocompleteClear() {
  listParam.value.name = ''
  listParam.value.id = '0'
  expandedKeys.value = []
  getList()
}
function autocompleteClearBlur() {
  const { id, key } = listParam.value
  if (!id || id === '0') {
    listParam.value.name = key
  }
  getList()
}
// 展开/收起全部
const allExpanded = ref(false)
function collectAllKeys(nodes: any[]): string[] {
  const keys: string[] = []
  const walk = (arr: any[]) => {
    arr.forEach((item: any) => {
      if (item.childs && item.childs.length) {
        keys.push(getRowKey(item))
        walk(item.childs)
      }
    })
  }
  walk(nodes)
  return keys
}
function toggleExpandAll() {
  if (allExpanded.value) {
    expandedKeys.value = [] as any
    allExpanded.value = false
  }
  else {
    expandedKeys.value = collectAllKeys(list.value) as any
    allExpanded.value = true
  }
}

const noFilterData = ref(false)
// 组织名称筛选
function handleFilter(inputValue: any, cb: any) {
  const arr: any = []
  noFilterData.value = false
  if (inputValue) {
    tileDepts.value.forEach((item: any) => {
      if (item.deptName.includes(inputValue)) {
        arr.push(item)
      }
    })
  }
  if (listParam.value.key) {
    const find = tileDepts.value.find((item: any) => item.ids === listParam.value.key)
    if (find) {
      arr.push(find)
    }
  }
  if (!arr.length) {
    noFilterData.value = true
    arr.push({ default: true })
  }
  cb(arr)
}
getList()
onMounted(() => {
  setSort()
})
</script>

<template>
  <div v-if="userStore.VA(['30101'])">
    <OrganizationOperate
      ref="organizationOperateRef" :title="orgTitle" :mode="orgMode" :current-item="currentItem" :list="deptsData"
      @add-org="addOrg" @update-org="updateOrg"
    />
    <page-main>
      <div class="header">
        <h4>{{ t('organization.title') }}</h4>
        <div class="header-right">
          <el-autocomplete
            ref="autocompleteRef" v-model="listParam.key" class="org-search"
            :placeholder="t('organization.namePlaceholder')" :trigger-on-focus="false" maxlength="50" clearable :fetch-suggestions="handleFilter"
            value-key="deptName" :popper-class="noFilterData ? 'el-autocomplete-nodata' : ''" :no-data-text="t('common.noMatchedData')"
            @select="deptChange"
            @clear="autocompleteClear"
            @input="listParam.id = '0';"
            @blur="autocompleteClearBlur"
            @keydown.enter="autocompleteClearBlur"
          >
            <template #prefix>
              <el-icon style="font-size: 14px;">
                <svg-icon name="ep:search" />
              </el-icon>
            </template>
            <template #default="{ item }">
              <div v-if="item.default">
                {{ t('empty.data') }}
              </div>
              <div v-else v-show-tip>
                <el-tooltip placement="top" :content="item.deptName">
                  <span class="ellipsis" style="width: 160px;">
                    {{ item.deptName }}
                  </span>
                </el-tooltip>
              </div>
            </template>
          </el-autocomplete>
          <el-button v-blur class="expand-btn" @click="toggleExpandAll">
            {{ allExpanded ? t('organization.collapseAll') : t('organization.expandAll') }}
          </el-button>
          <el-button v-if="userStore.VA(['30102'])" v-blur type="primary" @click="organizationOperateChange">
            <el-icon style="font-size: 10px;margin-right: 4px;">
              <svg-icon name="white-add" />
            </el-icon>
            {{ t('common.add') }}
          </el-button>
        </div>
      </div>
      <el-table
        id="dragTable"
        ref="tableRef"
        :data="list"
        style="width: 100%;"
        height="calc(100vh - 204px)"
        :expand-row-keys="expandedKeys"
        :row-key="getRowKey"
        :tree-props="{ children: 'childs' }"
      >
        <el-table-column prop="deptName" :label="t('organization.name')" show-overflow-tooltip min-width="220">
          <template #default="scope">
            <span class="move">
              <svg-icon name="move" style="font-size: 14px;width: 14px;height: 14px;" />
            </span>
            <el-icon class="oico">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
                <path v-if="scope.row.level === 1" d="M3 9l9-6 9 6v10a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z" stroke-linejoin="round" />
                <path v-else d="M3 7h6l2 2h10v10H3z" stroke-linejoin="round" />
              </svg>
            </el-icon>
            <span class="oname">{{ scope.row.deptName?.replace(/\s/g, '&nbsp;') }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="leader" :label="t('organization.leader')" show-overflow-tooltip min-width="140">
          <template #default="{ row }">
            {{ row.leader || '—' }}
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" :label="t('organization.createdAt')" show-overflow-tooltip min-width="180">
          <template #default="{ row }">
            <span class="mono num">{{ $toCustomDate(row.createdAt) }}</span>
          </template>
        </el-table-column>
        <el-table-column fixed="right" width="1px">
          <template #default="scope">
            <div class="el-table-btn acts">
              <a v-if="userStore.VA(['30102']) && scope.row.level !== 5" v-blur @click="addItemOpen(scope.row)">{{ t('common.addChild') }}</a>
              <a v-if="userStore.VA(['30103'])" v-blur @click="updateOrgOpen(scope.row)">{{ t('common.edit') }}</a>
              <a v-if="userStore.VA(['30104'])" v-blur class="del" @click="openDel(scope.row)">{{ t('common.delete') }}</a>
            </div>
          </template>
        </el-table-column>
        <template #empty>
          <TableSkeleton v-if="loading" :columns="['30%', '20%', '22%']" />
          <default-page
            v-else
            :no-permission="noPermission"
            :no-search-results="noSearchResults"
            :no-data="noData"
            :error="loadError"
            :show-add="false"
            :data-desc="t('organization.emptyDesc')"
            @reset="onResetSearch"
            @retry="getList()"
          />
        </template>
      </el-table>
    </page-main>
    <!-- 删除 -->
    <hint-dialog
      ref="hintRef" :title="t('app.tip')" :confirm="delOrg" :cancel="() => hintRef.dialogVisibleChange(false)"
      :content="t('common.deleteConfirm')"
    />
    <!-- 提示 -->
    <hint-dialog
      ref="hintDialogRef" :title="t('app.tip')" :close="() => hintRef.dialogVisibleChange(false)"
      :confirm="() => hintDialogRef.dialogVisibleChange(false)" :cancel="() => hintDialogRef.dialogVisibleChange(false)"
      :content="hintMsg" icon-class-name="icon-warning" :cancel-i-f="false" font-size="14px"
    />
  </div>
</template>

<style lang="scss" scoped>
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  min-height: 40px;
  margin-bottom: 18px;

  h4 {
    margin: 0;
    font-family: var(--font-display);
    font-size: 26px;
    font-weight: 620;
    letter-spacing: -0.012em;
    color: var(--fg);
  }

  .header-right {
    flex: none;
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .org-search {
    width: 220px;
  }
}

:deep(.el-table) {
  // 名称列：拖拽手柄(hover显隐) + 文件夹图标 + 名称
  .move {
    display: inline-flex;
    align-items: center;
    width: 16px;
    margin-right: 4px;
    color: var(--faint);
    opacity: 0;
    cursor: grab;
    transition: opacity 0.15s var(--ease);
    vertical-align: middle;
  }

  tr:hover .move {
    opacity: 1;
  }

  .oico {
    margin-right: 6px;
    color: var(--faint);
    vertical-align: middle;
  }

  .oname {
    font-weight: 550;
    color: var(--fg);
    vertical-align: middle;
  }
}
</style>
