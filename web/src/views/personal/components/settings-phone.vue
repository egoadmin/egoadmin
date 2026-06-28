<script lang="ts" setup>
import { t } from '@/i18n'

const emits = defineEmits(['editInfo'])
const dialogPhoneVisible = ref(false)
const form = ref({
  password: '',
  phone: '',
})

const formRef = ref()

const rules = {
  password: [
    {
      required: true,
      message: t('personal.loginPasswordRequired'),
      trigger: 'blur',
    },
  ],
  phone: [
    {
      required: true,
      pattern: /^1[3-9][0-9]{9}$/,
      message: t('personal.phoneInvalid'),
      trigger: 'blur',
    },
  ],
}
function dialogPhoneVisibleChange(val: boolean) {
  dialogPhoneVisible.value = val
}

function cancel() {
  formRef.value.resetFields()
  dialogPhoneVisibleChange(false)
}

function save() {
  formRef.value.validate((val: boolean) => {
    if (val) {
      emits('editInfo', { ...form.value })
    }
  })
}

defineExpose({
  dialogPhoneVisibleChange,
})
</script>

<template>
  <div>
    <el-dialog
      v-model="dialogPhoneVisible"
      class="phone-dialog"
      :close-on-click-modal="false"
      :title="t('personal.changePhone')"
      align-center
      width="440px"
      destroy-on-close
      @close="cancel"
    >
      <el-form ref="formRef" :model="form" label-position="top" :rules="rules" class="phone-form">
        <el-form-item :label="t('personal.phone')" prop="phone">
          <el-input
            v-model="form.phone"
            type="text"
            maxlength="11"
            :placeholder="t('user.phoneValidPlaceholder')"
            clearable
          />
        </el-form-item>

        <el-form-item :label="t('personal.loginPassword')" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            maxlength="20"
            :placeholder="t('personal.loginPasswordPlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button v-blur @click="cancel">{{ t('common.cancel') }}</el-button>
          <el-button v-blur type="primary" :disabled="!form.phone || !form.password" @click="save">
            {{ t('common.save') }}
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss">
.phone-dialog .el-dialog__body {
  padding: 22px;
}
</style>

<style lang="scss" scoped>
.phone-form {
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
