<script lang="ts" setup>
import { ElMessage } from 'element-plus'
import RoleOperate from './components/role-operate.vue'
import RoleDetails from './components/role-details.vue'
import apiRole from '@/api/modules/role'
import { menu } from '@/config/routeMenu'
import useUserStore from '@/store/modules/user'
import { t } from '@/i18n'
import { isNoPermissionMessage } from '@/utils/i18n'

const userStore = useUserStore()
// 数据范围标签映射（与 role-details / role-operate 一致）
const dataPermMap = new Map([[1, 'role.allData'], [2, 'role.orgAndBelow'], [3, 'role.orgOnly'], [4, 'role.selfOnly']])
const list = ref([])
const loading = ref(true)
const listParam = ref({
  limit: 10,
  name: '',
  sort: '',
  order: '',
  page: 1,
  total: 0,
  pageSizes: [10, 20, 30, 40, 50],

})
const roleOperateRef = ref()
const apiList = ref([])
const operateTitle = ref('')
const rows = ref({} as any)
const menuView = ref([])

const hintDialogRef = ref()
const hintDialogDelRef = ref()
const roleDetailsRef = ref()
const noSearchResults = ref(false)
const noData = ref(false)
const noPermission = ref(false)
const loadError = ref(false)
const hintMsg = ref('')
const hintDelRef = ref()
// 获取角色列表
async function getList(rest = false) {
  loading.value = true
  noSearchResults.value = false
  noData.value = false
  noPermission.value = false
  loadError.value = false
  list.value = []
  if (rest) {
    listParam.value.page = 1
  }
  try {
    const res: any = await apiRole.getRoleList(listParam.value)
    list.value = res.roles
    listParam.value.total = res.total
    loading.value = false
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
    loading.value = false
  }
}

// 空状态操作：清空搜索 / 重试
function onResetSearch() {
  listParam.value.name = ''
  getList(true)
}

// 新增角色
async function addRole(data: any) {
  await apiRole.addRole(data)
  ElMessage.success(t('role.addSuccess'))
  getList(true)
  roleOperateRef.value.dialogOperateVisibleChange(false)
}

// 打开编辑角色
async function updateRoleOpen(row: any) {
  operateTitle.value = 'role.editTitle'
  const res: any = await apiRole.getRole({ id: row.id })
  roleOperateRef.value.dialogOperateVisibleChange(true, res.role)
}
const updateData = ref()
// 编辑角色
async function updateRole(data: any) {
  // confirmUpdateRole()
  updateData.value = data
  hintDialogDelRef.value.dialogVisibleChange(true)
}
async function confirmUpdateRole() {
  await apiRole.updateRole(updateData.value)
  ElMessage.success(t('role.editSuccess'))
  getList()
  roleOperateRef.value.dialogOperateVisibleChange(false)
  if (roleDetailsRef.value.dialog) {
    roleDetailsOpen(updateData.value)
  }
  hintDialogDelRef.value.dialogVisibleChange(false)
}
// 新增--编辑打开
function roleOperateChange(row: any) {
  if (row?.update) {
    updateRoleOpen(row)
    return
  }
  operateTitle.value = 'role.addTitle'
  roleOperateRef.value.dialogOperateVisibleChange(true)
}

function delRoleOpen(row: any) {
  rows.value = row
  hintDialogRef.value.dialogVisibleChange(true)
}

// 删除角色
async function delRole() {
  const res: any = await apiRole.checkDeleteRole({ id: rows.value.id })
  if (res.isAllow) {
    await apiRole.deleteRole({ id: rows.value.id })
    ElMessage.success(t('role.deleteSuccess'))
    if (listParam.value.page > 1 && listParam.value.page) {
      if (listParam.value.total % listParam.value.limit === 1) {
        listParam.value.page -= 1
      }
    }
    getList()
    hintDialogRef.value.dialogVisibleChange(false)
  }
  else {
    hintDelRef.value.dialogVisibleChange(true)
    hintMsg.value = res.msg
  }
}

// 详情
async function roleDetailsOpen(row: any) {
  const res: any = await apiRole.getRole({ id: row.id })
  roleDetailsRef.value.dialogChange(true, res.role)
  return res.role
}

// 获取系统所有接口
async function getApiList() {
  const res: any = await apiRole.getAPIs()
  apiList.value = res.apis
  menuView.value = menuHandle()
  console.log(menuView.value)
}

// 处理菜单的权限值
function menuHandle() {
  const arr = JSON.parse(JSON.stringify(menu))
  // 根据角色权限过滤出接口
  const filterPath = (childs: any) => {
    childs.forEach((item: any) => {
      if (item.apiList && item) {
        const list = typeof item?.apiList === 'string' ? item?.apiList?.split(',') : []
        item.apiList = apisHandle(list)
      }
      if (item.child) {
        filterPath(item.child)
      }
    })
  }
  filterPath(arr)
  return arr
}
function apisHandle(api: any) {
  const apis = [...new Set(api)]
  const res: any[] = []
  apis.forEach((item: any) => {
    const find: any = apiList.value.find((row: any) => row.fullPath === item.toLocaleUpperCase())
    if (find) {
      res.push(find.id)
    }
  })
  return res
}

function onSizeChange(e: any) {
  console.log(e)
  listParam.value.limit = e
  listParam.value.page = 1
  getList()
}
function onCurrentChange(e: any) {
  listParam.value.page = e
  getList()
}

getList()
getApiList()
</script>

<template>
  <div v-if="userStore.VA(['20101'])">
    <RoleOperate ref="roleOperateRef" :title="operateTitle" :api-list="apiList" :menu-view="menuView" @add-role="addRole" @update-role="updateRole" />
    <RoleDetails ref="roleDetailsRef" @role-operate-change="roleOperateChange" />
    <page-main>
      <div class="header">
        <h4>{{ t('role.title') }}</h4>
        <div class="header-right">
          <el-input v-model="listParam.name" :placeholder="t('role.searchPlaceholder')" clearable maxlength="50" @clear="getList(true)" @keydown.enter="getList(true)" @blur="getList(true)">
            <template #prefix>
              <el-icon style="font-size: 14px;">
                <svg-icon name="ep:search" />
              </el-icon>
            </template>
          </el-input>
          <el-button v-if="userStore.VA(['20102'])" v-blur type="primary" @click="roleOperateChange">
            <el-icon style="font-size: 10px;margin-right: 4px;">
              <svg-icon name="white-add" />
            </el-icon>
            {{ t('common.add') }}
          </el-button>
        </div>
      </div>
      <el-table
        id="dragTable"
        :data="list"
        style="width: 100%;"
        row-key="id"
        height="calc(100vh - 204px)"
      >
        <el-table-column show-overflow-tooltip prop="name" :label="t('role.name')" min-width="140">
          <template #default="{ row }">
            <span class="rname" :class="{ clickable: userStore.VA(['20105']) }" @click="userStore.VA(['20105']) ? roleDetailsOpen(row) : ''">
              {{ row.name?.replace(/\s/g, '&nbsp;') || '—' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="desc" :label="t('role.desc')" min-width="220">
          <template #default="{ row }">
            <span :class="{ clickable: userStore.VA(['20105']) }" @click="userStore.VA(['20105']) ? roleDetailsOpen(row) : ''">
              {{ row.desc || '—' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="dataPerm" :label="t('role.dataPermission')" min-width="140">
          <template #default="{ row }">
            <span v-if="row.dataPerm" class="tag-neutral">{{ t(dataPermMap.get(row.dataPerm) || '') }}</span>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="createdAt" :label="t('role.createdAt')" min-width="140">
          <template #default="{ row }">
            <span class="mono num">{{ $toCustomDate(row.createdAt) }}</span>
          </template>
        </el-table-column>
        <el-table-column fixed="right" width="1px">
          <template #default="{ row }">
            <div class="el-table-btn acts">
              <a v-if="userStore.VA(['20103'])" v-blur @click="updateRoleOpen(row)">{{ t('common.edit') }}</a>
              <a v-if="userStore.VA(['20104'])" v-blur class="del" @click="delRoleOpen(row)">{{ t('common.delete') }}</a>
            </div>
          </template>
        </el-table-column>
        <template #append>
          <Pagination
            v-if="list.length"
            :page-state="{ ...listParam, currentPage: listParam.page, pageSize: listParam.limit }" :row-height="56" :sub-height="204 + 48"
            layout="total, sizes, prev, pager, next" :data-length="list.length"
            @size-change="onSizeChange" @current-change="onCurrentChange"
          />
        </template>
        <template #empty>
          <TableSkeleton v-if="loading" :columns="['22%', '40%', '14%', '18%']" />
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
    <hint-dialog ref="hintDialogRef" :title="t('app.tip')" :confirm="delRole" :cancel="() => hintDialogRef.dialogVisibleChange(false)" :content="t('common.deleteConfirm')" />
    <!-- 提示 -->
    <hint-dialog ref="hintDialogDelRef" icon-class-name="icon-warning" :title="t('app.tip')" :confirm="confirmUpdateRole" :cancel="() => hintDialogDelRef.dialogVisibleChange(false)" :content="t('role.editImpactConfirm')" />
    <!-- 提示失败 -->
    <hint-dialog ref="hintDelRef" :title="t('app.tip')" :close="() => hintDialogRef.dialogVisibleChange(false)" :confirm="() => hintDelRef.dialogVisibleChange(false)" :cancel="() => hintDelRef.dialogVisibleChange(false)" :content="hintMsg" icon-class-name="icon-warning" :cancel-i-f="false" font-size="14px" />
  </div>
</template>

<style lang="scss" scoped>
:deep(.el-tree) {
  & > .el-tree-node > .el-tree-node__content {
    padding-left: 8px !important;

    .el-tree-node__expand-icon {
      display: none;
    }
  }
}

.header {
  display: flex;
  align-items: center;
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

  .el-input {
    width: 220px;
    margin: 0 0 0 12px;
  }

  .header-right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 10px;
  }
}

.rname {
  font-weight: 550;
  color: var(--fg);
}

.clickable {
  cursor: pointer;

  &:hover {
    color: var(--accent-text);
  }
}

:deep(.el-tag) {
  border: none;
}

.el-checkbox-group {
  width: 208px;
  display: flex;
  color: var(--fg);
}

:deep(.el-drawer) {
  overflow: initial;
}
</style>
