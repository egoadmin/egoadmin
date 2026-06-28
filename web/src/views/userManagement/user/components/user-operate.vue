<script lang="ts" setup>
import { ref } from 'vue'
import { t } from '@/i18n'

const props = defineProps({
  orgList: {
    type: Array,
    default: () => [],
  },
  roleList: {
    type: Array,
    default: () => [],
  },
  title: {
    type: String,
    default: '',
  },
  mode: {
    type: String,
    default: 'add',
  },
})
const emits = defineEmits(['addUser', 'updateUser'])
const orgList: any = computed(() => props.orgList)
const roleList: any = computed(() => props.roleList)
const title: any = computed(() => props.title)
const isEdit = computed(() => props.mode === 'edit')
const formRef = ref()
const form = ref({
  username: '',
  name: '',
  gender: 1,
  deptId: '',
  phone: '',
  userStatus: 1,
  roleIds: '',
})
const rules = {
  username: [{
    required: true,
    trigger: 'blur',
    validator: (rule: any, value: any, callback: any) => {
      if (!value) {
        callback(new Error(t('user.usernameRequired')))
      }
      else if (!/^[0-9a-zA-z]{4,20}$/.test(value)) {
        callback(new Error(t('user.usernameInvalid')))
      }
      else {
        callback()
      }
    },
  }],
  name: [{
    required: true,
    trigger: 'blur',
    validator: (rule: any, value: any, callback: any) => {
      if (!value) {
        callback(new Error(t('user.nameRequired')))
      }
      else if (!/^[\u4E00-\u9FA5a-zA-Z]{2,20}$/.test(value)) {
        callback(new Error(t('user.nameInvalid')))
      }
      else {
        callback()
      }
    },
  }],
  phone: [{
    required: true,
    trigger: 'blur',
    validator: (rule: any, value: any, callback: any) => {
      if (!value) {
        callback(new Error(t('user.phoneRequired')))
      }
      else if (!/^1[3-9][0-9]{9}$/.test(value)) {
        callback(new Error(t('user.phoneInvalid')))
      }
      else {
        callback()
      }
    },
  }],
  roleIds: [{
    required: true, message: t('common.selectRequired'), trigger: 'change',
  }],
  deptId: [{
    required: true, message: t('common.selectRequired'), trigger: 'change',
  }],
}
const dialogOperateVisible = ref(false)

function dialogOperateVisibleChange(val: boolean, row?: any) {
  if (row) {
    form.value = { ...row, roleIds: row.roleIds[0] }
  }
  dialogOperateVisible.value = val
  // nextTick(() => {
  //   const cascader = document.querySelector('.radio-cascader')
  //   const radio = cascader?.querySelectorAll('.el-radio')
  //   const label = cascader?.querySelectorAll('.el-cascader-node__label')

  //   console.log(radio)
  // })
}

const cascaderProps = {
  checkStrictly: true,
  value: 'id',
  label: 'deptName',
  children: 'childs',
}

function save() {
  formRef.value.validate((valid: any) => {
    if (valid) {
      if (isEdit.value) {
        emits('updateUser', form.value)
      }
      else {
        emits('addUser', form.value)
      }
    }
  })
}

function close() {
  formRef.value.resetFields()
  form.value = {
    username: '',
    name: '',
    gender: 1,
    deptId: '',
    phone: '',
    userStatus: 1,
    roleIds: '',
  }
}

defineExpose({
  dialogOperateVisibleChange,
})
</script>

<template>
  <div>
    <el-dialog v-model="dialogOperateVisible" class="user-dialog" :close-on-click-modal="false" :title="title" align-center width="560px" @close="close">
      <el-form ref="formRef" :model="form" label-position="top" :rules="rules" class="form-grid">
        <el-form-item :label="t('user.username')" prop="username">
          <el-input v-model="form.username" type="text" :placeholder="t('user.usernameCreatePlaceholder')" maxlength="20" :disabled="isEdit" />
        </el-form-item>
        <el-form-item :label="t('user.name')" prop="name">
          <el-input
            v-model="form.name" type="text" :placeholder="t('user.realNamePlaceholder')"
            maxlength="20"
          />
        </el-form-item>
        <el-form-item :label="t('user.role')" prop="roleIds">
          <el-select v-if="dialogOperateVisible" v-model="form.roleIds" :placeholder="t('common.selectPlaceholder')">
            <el-option v-for="item in roleList" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('user.organization')" prop="deptId">
          <el-cascader
            v-if="dialogOperateVisible"
            v-model="form.deptId"
            :props="cascaderProps"
            :options="orgList"
            clearable
            :placeholder="t('common.selectPlaceholder')"
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
        <el-form-item :label="t('user.phone')" prop="phone">
          <el-input v-model="form.phone" type="text" :placeholder="t('user.phoneValidPlaceholder')" maxlength="11" />
        </el-form-item>
        <el-form-item :label="t('user.gender')">
          <el-radio-group v-model="form.gender">
            <el-radio :value="1" size="large">
              {{ t('common.male') }}
            </el-radio>
            <el-radio :value="2" size="large">
              {{ t('common.female') }}
            </el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="t('user.status')" class="full">
          <el-switch
            v-model="form.userStatus"
            width="52"
            inline-prompt
            :active-text="t('common.enabled')"
            :inactive-text="t('common.disabled')"
            :active-value="1"
            :inactive-value="2"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button v-blur @click="dialogOperateVisible = false">{{ t('common.cancel') }}</el-button>
          <el-button v-blur type="primary" :disabled="!form.username || !form.name || !form.roleIds || !form.deptId || !form.phone" @click="save">
            {{ t('common.save') }}
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss">
// 弹窗内边距对齐设计（22px）
.user-dialog .el-dialog__body {
  padding: 22px;
}
</style>

<style lang="scss" scoped>
// 两列栅格表单（设计 .dlg-b grid 1fr 1fr）
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px 18px;

  :deep(.el-form-item) {
    margin-bottom: 14px;

    &.full {
      grid-column: 1 / -1;
    }

    .el-form-item__label {
      padding-bottom: 6px;
      font-size: 13px;
      font-weight: 550;
      color: var(--text);
      line-height: 1.4;
    }

    .el-input,
    .el-select,
    .el-cascader {
      width: 100%;
    }
  }
}
</style>
