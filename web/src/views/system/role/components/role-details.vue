<script lang="ts" setup>
import { menu } from '@/config/routeMenu'
import useUserStore from '@/store/modules/user'
import { t } from '@/i18n'

const emit = defineEmits(['roleOperateChange'])
const userStore = useUserStore()
const dialog = ref(false)
const role: any = ref({})
const data: any = ref([])
const dataPerm = new Map([[1, 'role.allData'], [2, 'role.orgAndBelow'], [3, 'role.orgOnly'], [4, 'role.selfOnly']])
const treeProps = ref({
  value: 'id',
  label: 'title',
  children: 'child',
})
const expandedKeys: any = ref([])
function dialogChange(val: boolean, row?: any) {
  console.log(row, 'sss')

  if (row) {
    role.value = row
    const viewMenus = row.viewMenus.split(',')
    data.value = filterTreeById(menu, viewMenus)
    expandedKeys.value = data.value.map((item: any) => item.id)
  }
  dialog.value = val
}

function roleOperateChange() {
  emit('roleOperateChange', { ...role.value, update: true })
}

// id筛选显示权限树
function filterTreeById(data: any, targetIds: any) {
  const result = []
  for (let i = 0; i < data.length; i++) {
    const item = data[i]
    if (targetIds.includes(item.id.toString())) {
      result.push(item)
    }
    else if (item.child && Array.isArray(item.child)) {
      const subResult: any = filterTreeById(item.child, targetIds)
      if (subResult.length > 0) {
        result.push({
          ...item,
          child: subResult,
        })
      }
    }
  }
  return result
}
defineExpose({
  dialogChange,
  dialog,
  role,
})
</script>

<template>
  <el-drawer v-model="dialog" :title="t('role.detailTitle')" direction="rtl" size="76%" :with-header="false" :modal="false">
    <div class="flex">
      <h4 style="font-size: 16px;margin-bottom: 33px;">
        {{ t('role.detailTitle') }}
      </h4>
      <el-button v-if="userStore.VA(['20103'])" v-blur link type="primary" @click="roleOperateChange">
        <al-icon name="#icon-edit-1" style="font-size: 13px;margin-right: 4px;" />
        {{ t('common.edit') }}
      </el-button>
    </div>

    <ul style="padding-left: 16px;">
      <li>
        <div>{{ t('role.name') }}:</div>
        <div>{{ role.name }}</div>
      </li>
      <li>
        <div>{{ t('role.desc') }}:</div>
        <div>
          {{ role.desc || '—' }}
        </div>
      </li>
      <li>
        <div>{{ t('role.dataPermission') }}:</div>
        <div>{{ role.dataPerm ? t(dataPerm.get(role.dataPerm) || '') : '—' }}</div>
      </li>
    </ul>
    <h4 style="margin-top: 32px;">
      {{ t('role.functionPermission') }}
    </h4>
    <div class="packUp" @click="dialogChange(false)">
      <al-icon name="#icon-chevron-right" style="font-size: 19px;color: #fff;" />
    </div>
    <el-tree
      style="padding-left: 16px;" :data="data" :props="treeProps" node-key="id"
      :default-expanded-keys="expandedKeys"
    >
      <template #default="{ data: node }">
        <span>{{ t(node.title) }}</span>
      </template>
    </el-tree>
  </el-drawer>
</template>

<style lang="scss" scoped>
ul,
li {
  list-style: none;
  padding: 0;
  margin: 0;
}

.el-drawer__body {
  position: relative;
}

.flex {
  display: flex;
  align-items: flex-start;

  .el-button {
    margin-left: auto;
  }
}

li {
  display: flex;
  font-size: 14px;
  margin-bottom: 16px;

  & > div:first-child {
    width: 66px;
    color: var(--muted);
    flex-shrink: 0;
    margin-right: 24px;
  }
}

h4 {
  font-size: 14px;
  color: var(--fg);
  margin: 0;
  margin-bottom: 18px;
}

.packUp {
  position: absolute;
  top: 128px;
  left: -21.5px;
  width: 20px;
  height: 42px;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  cursor: pointer;
  background: var(--el-color-primary);
  transform: perspective(0.5em) rotateY(-6deg);
  border-radius: 4px 0 0 4px;
}
</style>
