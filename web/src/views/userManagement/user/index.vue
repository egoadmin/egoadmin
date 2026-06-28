<script lang="ts" setup>
import { ElMessage } from 'element-plus'
import UserOperate from './components/user-operate.vue'
import apiOrg from '@/api/modules/organization'
import apiUser from '@/api/modules/user'
import apiRole from '@/api/modules/role'
import useUserStore from '@/store/modules/user'
import useSettingsStore from '@/store/modules/settings'
import { t } from '@/i18n'
import { isNoPermissionMessage } from '@/utils/i18n'

const userStore = useUserStore()
const settingsStore = useSettingsStore()
// 获取组织列表
const orgList = ref([])
// 获取组织名称
const orgName = ref('')
const orgId = ref('')
const list = ref([])
const loading = ref(true)
const listParam = ref({
  page: 1,
  limit: 10,
  name: '',
  sort: '',
  order: '',
  userStatus: -1,
  roleId: '',
  deptId: '',
  total: 0,
  phone: '',
  pageSizes: [10, 20, 30, 40, 50],
  check: [] as number[],
  roleList: [] as any,
})
const tileDepts = ref([])
const treeProps = ref({
  value: 'id',
  label: 'deptName',
  children: 'childs',
})

const ids = ref<any[]>([])
const operateTitle = ref('')
const operateMode = ref<'add' | 'edit'>('add')
const hintRef = ref()
const hintDialogRef = ref()
const userOperateRef = ref()
const rows = ref({} as any)
const deptsData = ref([])
const treeRef = ref()
const scrollbarRef = ref()
const treeLoading = ref(true)
// 首次加载
const initialLoading = ref(true)
function dialogOperateVisibleChange() {
  operateTitle.value = t('user.addTitle')
  operateMode.value = 'add'
  userOperateRef.value.dialogOperateVisibleChange(true)
}

const tableRef = ref()
const selectionList = ref([])
// 表格选择
function selectionChange(e: any) {
  selectionList.value = e
}
function sortChange({ order, prop }: any) {
  listParam.value.order = order === 'descending' || !order ? 'desc' : 'asc'
  listParam.value.sort = prop
  getList()
}
// 清空表格选择
function clearSelection() {
  tableRef.value.clearSelection()
}

// 获取组织列表
async function getOrgList(fun?: any) {
  let name = orgName.value
  if (orgId.value && orgId.value !== '0') {
    name = ''
  } else if (orgName.value.length > 50) {
    orgName.value = orgName.value.slice(0, 50)
    name = orgName.value
  }
  treeLoading.value = true
  const res: any = await apiOrg.uGetDept({ name, id: orgId.value || '0' })
  // 是否首次加载
  if (initialLoading.value) {
    return res
  } else {
    treeLoading.value = false
    orgList.value = res.depts
    if (!orgName.value) {
      tileDepts.value = tileDeptsHandle(res.depts)
    }
    if (fun) {
      fun()
    }
  }
}
const noSearchResults = ref(false)
const noData = ref(false)
const noPermission = ref(false)
const loadError = ref(false)
// 计算用户列表空状态（有筛选/选中组织 → 无搜索结果，否则 → 无数据）
function computeUserEmpty() {
  if (list.value.length) {
    return
  }
  const hasFilter
    = !!listParam.value.name
      || !!listParam.value.phone
      || !!listParam.value.check.length
      || !!listParam.value.roleId
      || (!!listParam.value.deptId && listParam.value.deptId !== '0')
  noSearchResults.value = hasFilter
  noData.value = !hasFilter
}
// 获取列表
async function getList(rest = false) {
  if (rest) {
    listParam.value.page = 1
  }
  loading.value = true
  noSearchResults.value = false
  noData.value = false
  noPermission.value = false
  loadError.value = false
  const data = {
    ...listParam.value,
    roleId: listParam.value.roleId || '0',
    deptId: listParam.value.deptId || '0',
  }
  try {
    const res: any = await apiUser.getUserList(data)
    // 是否首次加载
    if (initialLoading.value) {
      return res
    } else {
      loading.value = false
      listParam.value.total = res.total
      list.value = res.users
      computeUserEmpty()
    }
  } catch (err: any) {
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

// 空状态操作：清空搜索/筛选
function onResetSearch() {
  listParam.value.name = ''
  listParam.value.phone = ''
  listParam.value.roleId = ''
  listParam.value.check = []
  listParam.value.deptId = '0'
  orgName.value = ''
  orgId.value = '0'
  treeRef.value?.setCurrentKey?.(null)
  getList(true)
}

// 获取角色列表
async function getRole() {
  const res: any = await apiRole.getRoleAll()
  listParam.value.roleList = res.roles
}
// 把组织平铺添加上级组织名
function tileDeptsHandle(depts: any) {
  const arr: any = []
  const tile = (data: any, name = '') => {
    data.forEach((item: any) => {
      arr.push({ deptName: name + item.deptName, id: item.id, level: item.level })
      if (item.childs) {
        tile(item.childs, `${name + item.deptName}>`)
      }
    })
  }
  tile(depts)
  return arr
}

// 新增角色
async function addUser(data: any) {
  const { deptId, roleIds } = data
  const param = {
    user: { ...data, deptId: deptId[deptId.length - 1], roleIds: [roleIds] },
    roleIds: [roleIds],
  }
  await apiUser.addUser(param)
  ElMessage.success(t('user.addSuccess'))
  userOperateRef.value.dialogOperateVisibleChange(false)
  getList(true)
}

// 编辑角色打开
async function updateUserOpen(row: any) {
  const res: any = await apiUser.getUser({ id: row.id })
  operateTitle.value = t('user.editTitle')
  operateMode.value = 'edit'
  userOperateRef.value.dialogOperateVisibleChange(true, res.user)
}

// 编辑角色
async function updateUser(data: any) {
  const { deptId, roleIds } = data
  const dId = typeof deptId === 'object' ? deptId[deptId.length - 1] : deptId
  const param = { ...data, deptId: dId, roleIds: [roleIds] }
  await apiUser.updateUser(param)
  if (param.id === userStore.userInfo.id) {
    userStore.getUserInfo()
  }
  ElMessage.success(t('user.editSuccess'))
  userOperateRef.value.dialogOperateVisibleChange(false)
  console.log(dId, listParam.value.deptId)

  if (
    listParam.value.deptId &&
    dId !== listParam.value.deptId &&
    listParam.value.page > 1 &&
    listParam.value.page
  ) {
    if (listParam.value.total % listParam.value.limit === 1) {
      listParam.value.page -= 1
    }
  }
  getList()
}

const batch = ref(false)
// 删除用户打开
function delUserOpen(isBatch: boolean, row?: any) {
  // 单个删除传入row
  if (!isBatch) {
    ids.value = [row.id]
  } else {
    const value = selectionList.value.map((item: any) => item.id)
    ids.value = value
  }
  batch.value = isBatch
  hintRef.value.dialogVisibleChange(true)
}
// 删除用户
async function delUser() {
  await apiUser.deleteUser({ ids: ids.value })
  ElMessage.success(t('user.deleteSuccess'))
  hintRef.value.dialogVisibleChange(false)
  if (listParam.value.page > 1 && listParam.value.page) {
    if (listParam.value.total % listParam.value.limit === ids.value.length) {
      listParam.value.page -= 1
    }
  }
  getList()
}

// 重置密码打开
function resetPasswordsOpen(row: any) {
  rows.value = row
  hintDialogRef.value.dialogVisibleChange(true)
}
// 重置密码
async function resetPasswords() {
  await apiUser.resetUserPassword({ id: rows.value.id })
  ElMessage.success(t('user.resetPasswordSuccess'))
  hintDialogRef.value.dialogVisibleChange(false)
  getList()
}
function checkboxChanga(e: any) {
  if (!e.length || e.length === 2) {
    listParam.value.userStatus = -1
  } else {
    listParam.value.userStatus = e[0]
  }
  getList()
}

// 组织点击时
function nodeClick(e: any) {
  listParam.value.deptId = e.id
  listParam.value.page = 1
  const find = tileDepts.value.find((item: any) => item.id === listParam.value.deptId)
  if (find) {
    deptsData.value = [find]
  }
  getList()
}

// 切换条数
function onSizeChange(e: any) {
  listParam.value.limit = e
  listParam.value.page = 1
  getList()
}

// 切换页数
function onCurrentChange(e: any) {
  listParam.value.page = e
  getList()
}

const autocompleteRef = ref()

// 组织输入收索选择时
function deptChange(node: any) {
  console.log(node)

  const data = node?.id || null
  listParam.value.deptId = data
  orgId.value = data || ''
  autocompleteRef.value.blur()
}
function autocompleteBlur() {
  getList()
  getOrgList(() => {
    nextTick(() => {
      treeRef.value.setCurrentKey(listParam.value.deptId, !!listParam.value.deptId)
    })
  })
}
const noFilterData = ref(false)
// 组织输入筛选
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
  if (!arr.length) {
    noFilterData.value = true
    arr.push({ default: true })
  }
  cb(arr)
}

Promise.all([getList(), getOrgList()])
  .then((res: any) => {
    initialLoading.value = false
    // 用户列表
    loading.value = false
    listParam.value.total = res[0].total
    list.value = res[0].users
    computeUserEmpty()
    // 组织信息
    treeLoading.value = false

    orgList.value = res[1].depts
    if (!orgName.value) {
      tileDepts.value = tileDeptsHandle(res[1].depts)
    }
  })
  .catch((err: any) => {
    loading.value = false
    treeLoading.value = false
    initialLoading.value = false
    if (isNoPermissionMessage(err?.message)) {
      noPermission.value = true
    }
    else {
      loadError.value = true
    }
  })
getRole()
</script>

<template>
  <div v-if="userStore.VA(['30201'])" id="pageMain" style="display: flex; margin: 0 16px">
    <div class="left-box">
      <div class="ctx-h">{{ t('user.orgScopeTitle') }}</div>
      <el-autocomplete
        ref="autocompleteRef"
        v-model="orgName"
        :placeholder="t('user.organizationPlaceholder')"
        :trigger-on-focus="false"
        maxlength="50"
        clearable
        :fetch-suggestions="handleFilter"
        value-key="deptName"
        :popper-class="noFilterData ? 'el-autocomplete-nodata' : ''"
        class="org-search"
        @select="deptChange"
        @clear="
          () => {
            orgName = ''
            orgId = '0'
            listParam.deptId = '0'
            autocompleteBlur()
          }
        "
        @blur="autocompleteBlur"
        @input="orgId = '0'"
        @keydown.enter="
          () => {
            orgId = '0'
            autocompleteBlur()
          }
        "
      >
        <template #default="{ item }">
          <div v-if="item.default">{{ t('empty.data') }}</div>
          <div v-else v-show-tip>
            <el-tooltip placement="top" :content="item.deptName">
              <span class="ellipsis" style="width: 160px">
                {{ item.deptName }}
              </span>
            </el-tooltip>
          </div>
        </template>
      </el-autocomplete>
      <el-scrollbar ref="scrollbarRef">
        <div v-if="treeLoading" class="tree-skeleton">
          <div v-for="(w, i) in ['70%', '52%', '60%', '44%', '58%']" :key="i" class="tree-sk-row" :style="{ paddingLeft: i === 0 ? '0' : '20px' }">
            <span class="tree-sk-ico" />
            <span class="tree-sk-bar" :style="{ width: w }" />
          </div>
        </div>
        <el-tree
          v-else
          ref="treeRef"
          class="custom-tree"
          :data="orgList"
          :props="treeProps"
          node-key="id"
          highlight-current
          default-expand-all
          @node-click="nodeClick"
        >
          <template #default="{ node }">
            <div class="tnode-label" v-show-tip>
              <el-icon class="tnode-ico">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
                  <path v-if="node.level === 1" d="M3 9l9-6 9 6v10a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z" stroke-linejoin="round" />
                  <path v-else d="M3 7h6l2 2h10v10H3z" stroke-linejoin="round" />
                </svg>
              </el-icon>
              <el-tooltip placement="top" :content="node.label">
                <span class="ellipsis tnode-name" :title="node.label">
                  {{ node.label }}
                </span>
              </el-tooltip>
            </div>
          </template>
        </el-tree>
      </el-scrollbar>
    </div>

    <UserOperate
      ref="userOperateRef"
      :org-list="orgList"
      :title="operateTitle"
      :mode="operateMode"
      :role-list="listParam.roleList"
      @add-user="addUser"
      @update-user="updateUser"
    />
    <page-main
      id="main"
      style="flex: 1"
      :style="{ marginLeft: settingsStore.isTabbarHorizontal ? '16px' : '1px', marginRight: 0 }"
    >
      <div class="header">
        <h4>{{ t('user.title') }}</h4>
        <div v-if="selectionList.length && userStore.VA(['30204'])" class="batchbar">
          <span class="sel">{{ t('common.selectedUsers', { count: selectionList.length }) }}</span>
          <el-button v-blur link type="primary" @click="clearSelection">
            {{ t('common.cancel') }}
          </el-button>
          <el-button v-blur class="batch-del" @click="delUserOpen(true)">
            <al-icon name="#icon-delete" style="margin-right: 4px" />
            {{ t('common.delete') }}
          </el-button>
        </div>
        <div class="header-right">
          <el-button
            v-if="userStore.VA(['30202'])"
            v-blur
            type="primary"
            @click="dialogOperateVisibleChange"
          >
            <el-icon style="font-size: 10px; margin-right: 4px">
              <svg-icon name="white-add" />
            </el-icon>
            {{ t('common.add') }}
          </el-button>
          <el-input
            v-model="listParam.name"
            :placeholder="t('user.usernameSearchPlaceholder')"
            clearable
            maxlength="50"
            @clear="getList(true)"
            @keydown.enter="getList(true)"
            @blur="getList(true)"
          >
            <template #prefix>
              <el-icon style="font-size: 14px">
                <svg-icon name="ep:search" />
              </el-icon>
            </template>
          </el-input>
          <Filtrator trigger-type="button" :checked-value="[listParam.roleId, listParam.check, listParam.phone]">
            <div style="color: var(--fg); margin-bottom: 10px">{{ t('user.status') }}</div>
            <el-checkbox-group v-model="listParam.check" @change="checkboxChanga">
              <el-checkbox size="large" :label="1">
                {{ t('common.enabled') }}
              </el-checkbox>
              <el-checkbox size="large" :label="2">
                {{ t('common.disabled') }}
              </el-checkbox>
            </el-checkbox-group>
            <div style="color: var(--fg); margin-bottom: 8px; margin-top: 16px">{{ t('user.phone') }}</div>
            <el-input
              v-model="listParam.phone"
              style="width: 366px; margin: 0"
              :placeholder="t('user.phonePlaceholder')"
              clearable
              maxlength="11"
              @clear="getList(true)"
              @keydown.enter="getList(true)"
              @blur="getList(true)"
            />
            <div style="color: var(--fg); margin-bottom: 8px; margin-top: 16px">{{ t('user.role') }}</div>
            <el-select
              v-model="listParam.roleId"
              style="width: 366px"
              clearable
              @change="getList(true)"
            >
              <el-option
                v-for="item in listParam.roleList"
                :key="item.id"
                :label="item.name"
                :value="item.id"
              />
            </el-select>
          </Filtrator>
        </div>
      </div>
      <el-table
        ref="tableRef"
        :data="list"
        style="width: 100%"
        row-key="id"
        height="calc(100vh - 204px)"
        @selection-change="selectionChange"
        @sort-change="sortChange"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column show-overflow-tooltip prop="username" :label="t('user.username')">
          <template #default="{ row }">
            <span class="uname mono">{{ row.username ? row?.username?.replace(/\s/g, '&nbsp;') : '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="name" :label="t('user.name')">
          <template #default="{ row }">
            {{ row.name ? row.name?.replace(/\s/g, '&nbsp;') : '—' }}
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="deptName" :label="t('user.organization')">
          <template #default="{ row }">
            {{ row.deptName ? row.deptName?.replace(/\s/g, '&nbsp;') : '—' }}
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="roleNames" :label="t('user.role')">
          <template #default="{ row }">
            <span v-if="row.roleNames && row.roleNames[0]" class="tag-neutral">{{ row.roleNames[0]?.replace(/\s/g, '&nbsp;') }}</span>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="phone" :label="t('user.phone')">
          <template #default="{ row }">
            <span class="mono num">{{ row.phone || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="userStatus" :label="t('user.status')" min-width="100">
          <template #default="{ row }">
            <span v-if="row.userStatus === 1" class="tag-status on">{{ t('common.enabled') }}</span>
            <span v-else class="tag-status off">{{ t('common.disabled') }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" :label="t('user.createdAt')" sortable width="200">
          <template #default="{ row }">
            <span class="mono num">{{ $toCustomDate(row.createdAt) }}</span>
          </template>
        </el-table-column>
        <el-table-column fixed="right" width="1px">
          <template #default="{ row }">
            <div class="el-table-btn acts">
              <a v-if="userStore.VA(['30203'])" v-blur @click="updateUserOpen(row)">{{ t('common.edit') }}</a>
              <a v-if="userStore.VA(['30205'])" v-blur @click="resetPasswordsOpen(row)">{{ t('role.resetPassword') }}</a>
              <a v-if="userStore.VA(['30204'])" v-blur class="del" @click="delUserOpen(false, row)">{{ t('common.delete') }}</a>
            </div>
          </template>
        </el-table-column>

        <template #append>
          <Pagination
            v-if="list.length"
            :page-state="{ ...listParam, currentPage: listParam.page, pageSize: listParam.limit }"
            :row-height="56"
            :sub-height="204 + 48"
            layout="total, sizes, prev, pager, next"
            :data-length="list.length"
            @size-change="onSizeChange"
            @current-change="onCurrentChange"
          />
        </template>
        <template #empty>
          <TableSkeleton v-if="loading" :columns="['14%', '14%', '16%', '14%', '14%', '12%']" />
          <default-page
            v-else
            :no-permission="noPermission"
            :no-search-results="noSearchResults"
            :no-data="noData"
            :error="loadError"
            :show-add="false"
            @reset="onResetSearch"
            @retry="getList()"
          />
        </template>
      </el-table>
    </page-main>
    <!-- 删除 -->
    <hint-dialog
      ref="hintRef"
      :title="t('app.tip')"
      :confirm="delUser"
      :cancel="() => hintRef.dialogVisibleChange(false)"
      :content="batch ? t('user.batchDeleteConfirm') : t('common.deleteConfirm')"
    />
    <!-- 重置密码 -->
    <hint-dialog
      ref="hintDialogRef"
      :title="t('app.tip')"
      :confirm="resetPasswords"
      icon-class-name="icon-warning"
      :cancel="() => hintDialogRef.dialogVisibleChange(false)"
      :content="t('user.resetPasswordConfirm', { username: rows.username })"
      font-size="14px"
    />
  </div>
</template>

<style lang="scss" scoped>
.left-box {
  width: 248px;
  height: calc(100vh - 96px);
  margin: 16px 0;
  padding: 14px;
  background-color: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--r-lg);
  overflow-y: auto;
  display: flex;
  flex-direction: column;

  .ctx-h {
    margin: 2px 4px 12px;
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
  }

  :deep(.org-search) {
    display: block;
    width: 100%;
    margin-bottom: 6px;
  }

  :deep(.el-scrollbar) {
    flex: 1;
    min-height: 0;
    height: auto !important;
  }

  .el-tree {
    width: 100%;
  }

  // 组织树骨架屏（替换旧 Lottie loading）
  .tree-skeleton {
    padding: 4px 0;

    .tree-sk-row {
      display: flex;
      align-items: center;
      gap: 8px;
      height: 36px;
    }

    .tree-sk-ico {
      flex: none;
      width: 16px;
      height: 16px;
      border-radius: 4px;
      background: var(--surface-2);
    }

    .tree-sk-bar {
      position: relative;
      height: 12px;
      overflow: hidden;
      background: var(--surface-2);
      border-radius: 6px;

      &::after {
        content: '';
        position: absolute;
        inset: 0;
        transform: translateX(-100%);
        background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--surface) 75%, transparent), transparent);
        animation: tree-sk-shimmer 1.3s infinite;
      }
    }
  }

  @keyframes tree-sk-shimmer {
    100% {
      transform: translateX(100%);
    }
  }

  .tnode-label {
    display: flex;
    align-items: center;
    gap: 7px;
    width: 100%;
    min-width: 0;

    .tnode-name {
      flex: 1;
      min-width: 0;
    }

    .tnode-ico {
      flex: none;
      color: var(--faint);
    }
  }

  :deep(.el-tree-node.is-current > .el-tree-node__content) .tnode-ico {
    color: var(--accent);
  }
}

.header {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 40px;
  margin-bottom: 18px;

  .batchbar {
    display: flex;
    align-items: center;
    gap: 14px;
    font-size: 13.5px;
    color: var(--muted);

    .sel {
      color: var(--muted);

      b {
        color: var(--accent-text);
      }
    }

    .batch-del {
      color: var(--danger);
      border-color: var(--border-strong);

      &:hover {
        color: var(--danger);
        background: var(--danger-weak);
        border-color: var(--danger);
      }
    }
  }

  h4 {
    margin: 0;
    font-family: var(--font-display);
    font-size: 26px;
    font-weight: 620;
    letter-spacing: -0.012em;
    color: var(--fg);
  }

  .el-input {
    width: 200px;
  }

  .header-right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 10px;
  }
}

.el-checkbox-group {
  display: flex;
  color: var(--fg);
}

// 用户名列：等宽 + 主文本色（设计 .uname）
.uname {
  color: var(--fg);
}

:deep(.el-tag) {
  border: none;
  border-radius: var(--r-pill);
}

// 组织树节点：圆角 hover/选中（设计 .tnode 圆角 r-sm）
:deep(.custom-tree) {
  --el-tree-node-content-height: 36px;

  .el-tree-node__content {
    border-radius: var(--r-sm);
  }
}

:deep(.custom-tree .is-current > .el-tree-node__content) {
  color: var(--accent);
  font-weight: 550;
  background-color: var(--accent-weak);
}
</style>
