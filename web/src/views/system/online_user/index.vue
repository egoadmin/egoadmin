<script lang="ts" setup>
import { ElMessage } from 'element-plus'
import apiUser from '@/api/modules/user'
import useUserStore from '@/store/modules/user'
import { t } from '@/i18n'
import { isNoPermissionMessage } from '@/utils/i18n'

const userStore = useUserStore()

const list = ref([])
const listParam = ref({
  order: 'desc',
  name: '',
  page: 1,
  limit: 10,
  sort: 'last_login_at',
  lastLoginIp: '',
  pageSizes: [10, 20, 30, 40, 50],
  total: 0,
})
const tableRef = ref()
const selectionList = ref([])
const loading = ref(true)
const ids = ref<any[]>([])
const batch = ref(false)
const noSearchResults = ref(false)
const noData = ref(false)
const noPermission = ref(false)
const loadError = ref(false)
const hintDialogRef = ref()
async function getList(rest = false) {
  if (rest) {
    listParam.value.page = 1
  }
  loading.value = true
  noSearchResults.value = false
  noData.value = false
  noPermission.value = false
  loadError.value = false
  list.value = []
  try {
    const res: any = await apiUser.getOnlineUserList(listParam.value)
    list.value = res.users
    listParam.value.total = res.total
    const { name, lastLoginIp } = listParam.value
    loading.value = false
    if (!list.value.length) {
      const hasFilter = !!name || !!lastLoginIp
      noSearchResults.value = hasFilter
      noData.value = !hasFilter
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

// 空状态操作：清空搜索/筛选
function onResetSearch() {
  listParam.value.name = ''
  listParam.value.lastLoginIp = ''
  getList(true)
}
// 表格选择
function selectionChange(e: any) {
  selectionList.value = e
}
// 清空表格选择
function clearSelection() {
  tableRef.value.clearSelection()
}
// 强退打开
async function forcedRetreatOpen(isBatch: boolean, row?: any) {
  // 单个删除传入row
  if (!isBatch) {
    ids.value = [row.id]
  }
  else {
    const value = selectionList.value.map((item: any) => item.id)
    ids.value = value
  }
  batch.value = isBatch
  hintDialogRef.value.dialogVisibleChange(true)
}

// 强制退出
async function forcedRetreat() {
  await apiUser.offlineUser({ ids: ids.value })
  ElMessage.success(t('onlineUser.forceLogoutSuccess'))
  hintDialogRef.value.dialogVisibleChange(false)
  if (listParam.value.page > 1 && listParam.value.page) {
    if (listParam.value.total % listParam.value.limit === ids.value.length) {
      listParam.value.page -= 1
    }
  }
  getList()
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

function sortChange({ order, prop }: any) {
  listParam.value.order = (order === 'descending' || !order) ? 'desc' : 'asc'
  listParam.value.sort = prop
  getList()
}

getList()
</script>

<template>
  <div v-if="userStore.VA(['20201'])" style="display: flex;">
    <page-main style="flex: 1;">
      <div class="header">
        <h4>{{ t('onlineUser.title') }}</h4>
        <div v-if="selectionList.length && userStore.VA(['20202'])" class="batchbar">
          <span class="sel">{{ t('common.selectedUsers', { count: selectionList.length }) }}</span>
          <el-button v-blur link type="primary" @click="clearSelection">
            {{ t('common.cancel') }}
          </el-button>
          <el-button v-blur class="batch-kick" @click="forcedRetreatOpen(true)">
            <el-icon class="kick-ico"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M16 17l5-5-5-5M21 12H9M9 5H5a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h4" stroke-linecap="round" stroke-linejoin="round" /></svg></el-icon>
            {{ t('onlineUser.forceLogout') }}
          </el-button>
        </div>
        <div class="header-right">
          <el-input v-model="listParam.name" :placeholder="t('onlineUser.usernamePlaceholder')" clearable maxlength="50" @clear="getList(true)" @keydown.enter="getList(true)" @blur="getList(true)">
            <template #prefix>
              <el-icon style="font-size: 14px;">
                <svg-icon name="ep:search" />
              </el-icon>
            </template>
          </el-input>
          <Filtrator trigger-type="button" :checked-value="[listParam.lastLoginIp]">
            <div class="filter-label">{{ t('onlineUser.loginIp') }}</div>
            <el-input v-model="listParam.lastLoginIp" :placeholder="t('onlineUser.loginIpPlaceholder')" maxlength="50" clearable @clear="getList(true)" @keydown.enter="getList(true)" @blur="getList(true)" />
          </Filtrator>
        </div>
      </div>
      <el-table
        ref="tableRef"
        :data="list"
        style="width: 100%;"
        row-key="id"
        height="calc(100vh - 204px)"
        @selection-change="selectionChange"
        @sort-change="sortChange"
      >
        <el-table-column type="selection" width="55" />
        <!-- <el-table-column type="index" label="序号" width="70" /> -->
        <el-table-column show-overflow-tooltip prop="username" :label="t('onlineUser.username')" min-width="140">
          <template #default="{ row }">
            <span class="uname mono">{{ row.username?.replace(/\s/g, '&nbsp;') || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="lastLoginIp" :label="t('onlineUser.loginIp')" min-width="140">
          <template #default="{ row }">
            <span class="mono num">{{ row.lastLoginIp || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="lastLoginAt" :label="t('onlineUser.loginTime')" sortable min-width="140">
          <template #default="{ row }">
            <span class="mono num">{{ $toCustomDate(row.lastLoginAt) }}</span>
          </template>
        </el-table-column>
        <el-table-column fixed="right" width="1px">
          <template #default="{ row }">
            <div v-if="userStore.VA(['20202'])" class="el-table-btn acts">
              <a v-blur class="del" @click="forcedRetreatOpen(false, row)">
                <el-icon class="kick-ico"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M16 17l5-5-5-5M21 12H9M9 5H5a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h4" stroke-linecap="round" stroke-linejoin="round" /></svg></el-icon>
                {{ t('onlineUser.forceLogout') }}
              </a>
            </div>
          </template>
        </el-table-column>
        <template #empty>
          <TableSkeleton v-if="loading" :columns="['18%', '28%', '24%']" />
          <default-page
            v-else
            :no-permission="noPermission"
            :no-search-results="noSearchResults"
            :no-data="noData"
            :error="loadError"
            :show-add="false"
            :data-desc="t('onlineUser.emptyDesc')"
            @reset="onResetSearch"
            @retry="getList()"
          />
        </template>
        <template #append>
          <Pagination
            v-if="list.length"
            :page-state="{ ...listParam, currentPage: listParam.page, pageSize: listParam.limit }" :row-height="56" :sub-height="204 + 48"
            layout="total, sizes, prev, pager, next" :data-length="list.length"
            @size-change="onSizeChange" @current-change="onCurrentChange"
          />
        </template>
      </el-table>
    </page-main>
    <!-- 强退 -->
    <hint-dialog
      ref="hintDialogRef"
      :title="t('app.tip')"
      :confirm="forcedRetreat"
      :cancel="() => hintDialogRef.dialogVisibleChange(false)"
      :content="batch ? t('onlineUser.batchForceLogoutConfirm') : t('onlineUser.forceLogoutConfirm')"
    />
    <!-- 提示 -->
  </div>
</template>

<style lang="scss" scoped>
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

    .batch-kick {
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

// 用户名列：等宽 + 主文本色（设计 .uname）
.uname {
  color: var(--fg);
}

// 筛选浮层字段标签
.filter-label {
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

// 行内强退操作：图标+文字对齐
:deep(.el-table-btn.acts a) {
  display: inline-flex;
  align-items: center;
}

.kick-ico {
  margin-right: 5px;
  font-size: 15px;
}
</style>
