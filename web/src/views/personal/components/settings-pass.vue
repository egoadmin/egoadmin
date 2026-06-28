<script lang="ts" setup>
import { ref } from 'vue'
import { t } from '@/i18n'

const emits = defineEmits(['editPassword'])
const formRef = ref()

const dialogFormVisible = ref(false)
const form = ref({
  oldPassword: '',
  password: '',
  confirmPass: '',
})

function validatePass(rule: any, value: any, callback: any) {
  if (!value) {
    callback(new Error(t('personal.newPasswordRequired')))
  }
  else if (!/^[a-zA-z0-9]{6,20}$/.test(value)) {
    callback(new Error(t('personal.newPasswordInvalid')))
  }
  else {
    callback()
  }
}

function validateConfirmPass(rule: any, value: any, callback: any) {
  const { password, confirmPass } = form.value
  if (!value) {
    callback(new Error(t('personal.confirmPasswordRequired')))
  }
  else if (password !== confirmPass) {
    callback(new Error(t('personal.passwordMismatch')))
  }
  else {
    callback()
  }
}

const rules = {

  oldPassword: [{
    required: true,
    message: t('personal.oldPasswordRequired'),
    trigger: 'blur',
  }],
  password: [{
    required: true,
    validator: validatePass,
    trigger: 'blur',
  }],
  confirmPass: [{
    required: true,
    validator: validateConfirmPass,
    trigger: 'blur',
  }],
}

function dialogFormVisibleChange(val: boolean) {
  dialogFormVisible.value = val
}

function cancel() {
  formRef.value.resetFields()
  dialogFormVisibleChange(false)
}

function save() {
  formRef.value.validate((val: boolean) => {
    if (val) {
      emits('editPassword', form.value)
    }
  })
}
defineExpose({
  dialogFormVisibleChange,
})
</script>

<template>
  <div>
    <el-dialog v-model="dialogFormVisible" class="pwd-dialog" :close-on-click-modal="false" :title="t('personal.changePassword')" align-center width="440px" destroy-on-close @close="cancel">
      <el-form ref="formRef" :model="form" label-position="top" :rules="rules" class="pwd-form">
        <el-form-item :label="t('personal.oldPassword')" prop="oldPassword">
          <el-input v-model="form.oldPassword" type="password" show-password maxlength="20" :placeholder="t('personal.oldPasswordPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('personal.newPassword')" prop="password">
          <el-input v-model="form.password" type="password" show-password maxlength="20" :placeholder="t('personal.newPasswordPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('personal.confirmPassword')" prop="confirmPass">
          <el-input v-model="form.confirmPass" type="password" show-password maxlength="20" :placeholder="t('personal.confirmPasswordPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button v-blur @click="cancel">{{ t('common.cancel') }}</el-button>
          <el-button v-blur type="primary" :disabled="!form.confirmPass || !form.oldPassword || !form.password" @click="save">
            {{ t('common.save') }}
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.pwd-dialog .el-dialog__body {
  padding: 22px;
}
</style>

<style lang="scss" scoped>
.pwd-form {
  :deep(.el-form-item) {
    margin-bottom: 16px;

    .el-form-item__label {
      padding-bottom: 6px;
      font-size: 13px;
      font-weight: 550;
      color: var(--text);
      line-height: 1.4;
    }

    .el-input {
      width: 100%;
    }
  }
}
</style>
