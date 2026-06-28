<script lang="ts" setup>
import apiUser from '@/api/modules/user'
import useUserStore from '@/store/modules/user'
import { t } from '@/i18n'
import { isNoPermissionMessage } from '@/utils/i18n'

const userStore = useUserStore()
const list = ref([])
const loading = ref(true)
const listParam = reactive({
  page: 1,
  limit: 10, // 一页的大小
  total: 1,
  pageSizes: [10, 20, 30, 40, 50],
  event: '',
  username: '',
  timeRange: '',
})

const noSearchResults = ref(false)
const noData = ref(false)
const noPermission = ref(false)
const loadError = ref(false)

// 登录/登出类操作用 Go 青信号色标签（设计 .tag.login）
const loginTypes = ['登录', '登出', '退出', 'login', 'logout']
function isLoginType(typ?: string) {
  return !!typ && loginTypes.some(k => typ.includes(k))
}
async function getList(rest = false) {
  if (rest) {
    listParam.page = 1
  }
  loading.value = true
  noSearchResults.value = false
  noData.value = false
  noPermission.value = false
  loadError.value = false
  list.value = []
  const data = JSON.parse(JSON.stringify(listParam))

  if (!data.timeRange) {
    data.timeRange = undefined
  }
  else {
    data.timeRange = {
      start: data.timeRange[0],
      end: data.timeRange[1],
    }
  }
  try {
    const res: any = await apiUser.getLogList(data)
    list.value = res.logs
    listParam.total = +res.count
    loading.value = false
    if (!list.value.length) {
      const { event, timeRange, username } = listParam
      const hasFilter = !!event || !!username || !!timeRange
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
  listParam.event = ''
  listParam.username = ''
  listParam.timeRange = '' as any
  getList(true)
}
getList()

function onSizeChange(e: any) {
  console.log(e)
  listParam.limit = e

  listParam.page = 1
  getList()
}
function onCurrentChange(e: any) {
  listParam.page = e
  getList()
}
onMounted(() => {
  getList()
})
</script>

<template>
  <div v-if="userStore.VA(['20301'])" style="display: flex;">
    <page-main style="flex: 1;">
      <div class="header">
        <h4>{{ t('operationLog.title') }}</h4>

        <div class="header-right">
          <el-input
            v-model="listParam.event" :placeholder="t('operationLog.eventPlaceholder')" clearable maxlength="50" @clear="getList(true)"
            @keydown.enter="getList(true)" @blur="getList(true)"
          >
            <template #prefix>
              <el-icon style="font-size: 14px;">
                <svg-icon name="ep:search" />
              </el-icon>
            </template>
          </el-input>
          <Filtrator trigger-type="button" :checked-value="[listParam.username, listParam.timeRange]">
            <div style="color: var(--fg);margin-bottom: 10px;">
              {{ t('operationLog.operator') }}
            </div>
            <el-input
              v-model="listParam.username" :placeholder="t('operationLog.operatorPlaceholder')" clearable style="width: 366px;margin: 0;"
              maxlength="50" @clear="getList(true)" @keydown.enter="getList(true)" @blur="getList(true)"
            >
              <template #suffix>
                <el-icon style="font-size: 14px;  cursor: pointer;">
                  <svg-icon name="ep:search" />
                </el-icon>
              </template>
            </el-input>
            <div style="color: var(--fg);margin-bottom: 8px;margin-top: 20px;">
              {{ t('operationLog.operationTime') }}
            </div>
            <el-date-picker
              v-model="listParam.timeRange" style="width: 366px;" :default-time="[
                new Date(2000, 1, 1, 0, 0, 0),
                new Date(2000, 2, 1, 23, 59, 59),
              ]" type="datetimerange" :start-placeholder="t('operationLog.startTime')" :end-placeholder="t('operationLog.endTime')" @change="getList(true)"
            />
          </Filtrator>
        </div>
      </div>
      <el-table :data="list" style="width: 100%;" row-key="id" height="calc(100vh - 204px)">
        <el-table-column show-overflow-tooltip prop="moduleName" :label="t('operationLog.moduleName')" min-width="140" />
        <el-table-column prop="typ" :label="t('operationLog.type')" min-width="100">
          <template #default="{ row }">
            <span v-if="row.typ" :class="isLoginType(row.typ) ? 'tag-info' : 'tag-neutral'">{{ row.typ }}</span>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="title" :label="t('operationLog.event')" min-width="200">
          <template #default="{ row }">
            <span class="ev">{{ row.title || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="username" :label="t('operationLog.operator')" min-width="120">
          <template #default="{ row }">
            <span class="mono">{{ row.username || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="deptName" :label="t('operationLog.deptName')" min-width="140">
          <template #default="{ row }">
            {{ row.deptName || '—' }}
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="clientIp" :label="t('operationLog.clientIp')" min-width="140">
          <template #default="{ row }">
            <span class="mono num">{{ row.clientIp || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column show-overflow-tooltip prop="updatedAt" :label="t('operationLog.operationTime')" width="200">
          <template #default="{ row }">
            <span class="mono num">{{ $toCustomDate(row.updatedAt) }}</span>
          </template>
        </el-table-column>
        <template #append>
          <Pagination
            v-if="list.length"
            :page-state="{ ...listParam, currentPage: listParam.page, pageSize: listParam.limit }" :row-height="56"
            :sub-height="204 + 48" layout="total, sizes, prev, pager, next" :data-length="list.length"
            @size-change="onSizeChange" @current-change="onCurrentChange"
          />
        </template>
        <template #empty>
          <TableSkeleton v-if="loading" :columns="['14%', '10%', '30%', '12%']" />
          <default-page
            v-else
            :no-permission="noPermission"
            :no-search-results="noSearchResults"
            :no-data="noData"
            :error="loadError"
            :show-add="false"
            :data-desc="t('operationLog.emptyDesc')"
            @reset="onResetSearch"
            @retry="getList()"
          />
        </template>
      </el-table>
      <!-- <span>{{ listParam.page }}-syy-{{ listParam.limit }}</span> -->
    </page-main>
  </div>
</template>

<style lang="scss" scoped>
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

// 事件内容列：主文本色（设计 .tbl .ev）
.ev {
  color: var(--fg);
}
</style>
