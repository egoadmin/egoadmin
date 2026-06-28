<script lang="ts" setup>
import { t } from '@/i18n'

const props = defineProps({
  title: {
    type: String,
    default: '',
  },
  mode: {
    type: String,
    default: 'add',
  },
  list: {
    type: Array,
    default: () => [],
  },
  currentItem: {
    type: Object || null,
    default: () => {},
  },
})
const emits = defineEmits(['addOrg', 'updateOrg'])
const title = computed(() => props.title)
const isEdit = computed(() => props.mode === 'edit')
const list: any = computed(() => {
  const data = filterData(props.list, 4)
  console.log(data,
  )

  return data
})
const dialogOperateVisible = ref(false)
const formRef = ref()
const form = ref({
  parentId: '',
  name: '',
  editId: '',
})
function dialogOperateVisibleChange(val: boolean) {
  dialogOperateVisible.value = val
}

const cascaderProps = {
  checkStrictly: true,
  value: 'id',
  label: 'deptName',
  children: 'childs',
}
const rules = {
  required: true,
  trigger: 'blur',
  validator: (rule: any, value: any, callback: any) => {
    if (!value) {
      callback(new Error(t('organization.nameRequired')))
    }
    else if (/^\s+$/.test(value)) {
      callback(new Error(t('organization.nameNotAllSpace')))
    }
    else {
      callback()
    }
  },
}

// 数组嵌套层级筛选
function filterData(data: any, level: any) {
  return data.map((item: any) => {
    const newItem: any = { id: item.id, deptName: item.deptName }
    if (item.childs && level > 1) {
      newItem.childs = filterData(item.childs, level - 1)
    }
    return newItem
  })
}

watch(() => props.currentItem, (value: any) => {
  const { id, deptName, editId } = value || {}
  form.value.parentId = id || ''
  form.value.name = deptName || ''
  form.value.editId = editId || ''
}, { deep: true })

function save() {
  formRef.value.validate((valid: boolean) => {
    if (valid) {
      const { name, parentId, editId }: any = form.value
      let id = parentId
      if (parentId) {
        // 数据返回是数组时只取最后一级
        id = typeof parentId === 'object' ? parentId[parentId.length - 1] : parentId
      }
      else {
        id = '0'
      }
      const data: any = {
        parentId: id,
        dept: {
          deptName: name,
        },
      }
      if (!isEdit.value) {
        emits('addOrg', data)
      }
      else {
        data.id = editId
        emits('updateOrg', data)
      }
    }
  })
}

function close() {
  formRef.value.resetFields()
  form.value = {
    parentId: '',
    name: '',
    editId: '',
  }
}
defineExpose({
  dialogOperateVisibleChange,
})
</script>

<template>
  <div>
    <el-dialog v-model="dialogOperateVisible" class="org-dialog" :close-on-click-modal="false" :title="title" align-center width="480px" @close="close">
      <el-form ref="formRef" :model="form" label-position="top" class="org-form">
        <el-form-item :label="t('organization.parent')">
          <el-cascader
            v-if="dialogOperateVisible"
            v-model="form.parentId"
            :disabled="isEdit"
            :props="cascaderProps" :options="list" clearable :placeholder="t('organization.parentPlaceholder')"
          >
            <template #default="{ node }">
              <div v-show-tip>
                <el-tooltip placement="top" :content="node.label">
                  <span class="ellipsis" style="width: 170px;">{{ node.label }}</span>
                </el-tooltip>
              </div>
            </template>
          </el-cascader>
        </el-form-item>

        <el-form-item :label="t('organization.name')" :rules="rules" prop="name">
          <el-input v-model="form.name" type="text" :placeholder="t('organization.namePlaceholder')" maxlength="30" />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button v-blur @click="dialogOperateVisible = false">{{ t('common.cancel') }}</el-button>
          <el-button v-blur type="primary" :disabled="!form.name" @click="save">
            {{ t('common.save') }}
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.org-dialog .el-dialog__body {
  padding: 22px;
}
</style>

<style lang="scss" scoped>
.org-form {
  :deep(.el-form-item) {
    margin-bottom: 14px;

    .el-form-item__label {
      padding-bottom: 6px;
      font-size: 13px;
      font-weight: 550;
      color: var(--text);
      line-height: 1.4;
    }

    .el-input,
    .el-cascader {
      width: 100%;
    }
  }
}
</style>
