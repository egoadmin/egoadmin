<script lang="ts" setup>
import { ElMessage } from 'element-plus'
import RoleStep from './role-step.vue'
import { t } from '@/i18n'

const props = defineProps({
  title: {
    type: String,
    default: '',
  },
  menuView: {
    type: Array,
    default: () => [],
  },
})
const emits = defineEmits(['addRole', 'updateRole'])

interface RoleForm {
  id?: number
  name: string
  desc: string
  dataPerm: number
  viewMenus: number[]
  expandedKeys: number[]
  apis: number[]
  typ: number
}

const formRef = ref()
const form = ref<RoleForm>({
  name: '',
  desc: '',
  dataPerm: 1,
  viewMenus: [1],
  expandedKeys: [1],
  apis: [],
  typ: 1,
})
const titleKey = computed(() => props.title || 'role.addTitle')
const title = computed(() => t(titleKey.value))
const menuView = computed(() => {
  form.value.expandedKeys = props.menuView?.map((item: any) => item.id)

  return props.menuView
})
const dialogOperateVisible = ref(false)
const treeProps = ref({
  value: 'id',
  label: 'title',
  children: 'child',
})

const rules = ref({
  name: [
    {
      required: true,
      trigger: 'blur',
      validator: (rule: any, value: any, callback: any) => {
        if (!value) {
          callback(new Error(t('role.nameRequired')))
        } else if (/^\s+$/.test(value)) {
          callback(new Error(t('role.nameNotAllSpace')))
        } else {
          callback()
        }
      },
    },
  ],
})

const stepList = ['role.stepBase', 'role.stepPermission']
const selectedStep = ref(['role.stepBase'])
const step = ref(1)
const treeRef = ref()
const isCheck = ref(true)
function stepChange(num: number) {
  step.value = num
}

function dialogOperateVisibleChange(val: boolean, row?: any) {
  dialogOperateVisible.value = val

  nextTick(() => {
    let check: any = []
    if (row) {
      const { name, desc, dataPerm, viewMenus, id } = row
      check = JSON.parse(JSON.stringify(viewMenus?.split(',') || []))
      isCheck.value = check.length <= 1
      form.value = {
        ...form.value,
        name,
        desc,
        dataPerm,
        id,
      }
      selectedStep.value = stepList
    }
    treeRef.value.setCheckedKeys(check)
  })
}

// 处理node选择节点
function handleNodeCheck() {
  const nodeValue: any[] = treeRef.value.getCheckedNodes()
  const viewMenus: any[] = []
  const apis: any[] = []
  if (nodeValue.length < 1) {
    ElMessage.warning(t('role.selectPermission'))
    return
  }

  nodeValue.forEach((item: any) => {
    viewMenus.push(item.id)
    apis.push(...(item.apiList || ''))
  })

  const nodeList = [...new Set(viewMenus)]

  return {
    viewMenus: nodeList.join(),
    apis: [...new Set(apis)],
  }
}

// 是否最后一步
const lastStep = computed(() => step.value === stepList.length)
// 下一步-保存
async function nextStepAndSave() {
  formRef.value.validate((valid: boolean) => {
    if (valid) {
      if (lastStep.value) {
        const nodeValue = handleNodeCheck()
        if (!nodeValue) {
          return
        }
        const data: any = { ...form.value, viewMenus: nodeValue?.viewMenus, apis: nodeValue?.apis }

        if (titleKey.value === 'role.addTitle') {
          emits('addRole', { role: data })
        } else {
          emits('updateRole', { id: data.id, role: data })
        }
      } else {
        step.value += 1
        selectedStep.value = stepList
      }
    }
  })
}
// 上一步-取消
function previousStepAndCancel() {
  if (lastStep.value) {
    step.value -= 1
    const check = treeRef.value.getCheckedKeys()
    treeRef.value.setCheckedKeys(check)
  } else {
    dialogOperateVisible.value = false
  }
}

// 弹框关闭
function dialogClose() {
  step.value = 1
  selectedStep.value = ['role.stepBase']
  formRef.value.resetFields()
  form.value = {
    name: '',
    desc: '',
    dataPerm: 1,
    viewMenus: [1],
    expandedKeys: [1],
    apis: [],
    typ: 1,
  }
}

// 树形复选框点击时
function nodeClick(e: any, { checkedKeys }: any) {
  console.log(checkedKeys)
  treeRef.value.setCheckedKeys(checkedKeys)
  isCheck.value = checkedKeys.length <= 1
}

const disabled = computed(() => {
  return (!form.value.name && step.value === 1) || (step.value === 2 && isCheck.value)
})

defineExpose({
  dialogOperateVisibleChange,
})
</script>

<template>
  <div>
    <el-dialog
      v-model="dialogOperateVisible"
      :close-on-click-modal="false"
      :title="title"
      align-center
      width="680px"
      @close="dialogClose"
    >
      <RoleStep
        :list="stepList"
        :selected-step="selectedStep"
        :step="step"
        class="step"
        @step-change="stepChange"
      />
      <el-form v-show="step === 1" ref="formRef" :model="form" label-position="top" :rules="rules" class="role-form">
        <el-form-item :label="t('role.name')" prop="name">
          <el-input
            v-model="form.name"
            type="text"
            :placeholder="t('role.nameRequired')"
            maxlength="30"
            clearable
          />
        </el-form-item>

        <el-form-item :label="t('role.desc')" prop="desc">
          <el-input
            v-model="form.desc"
            show-word-limit
            type="textarea"
            :placeholder="t('role.descPlaceholder')"
            maxlength="200"
          />
        </el-form-item>

        <el-form-item :label="t('role.dataPermission')">
          <el-radio-group v-model="form.dataPerm">
            <el-radio :value="1" size="large">{{ t('role.allData') }}</el-radio>
            <el-radio :value="3" size="large">{{ t('role.orgOnly') }}</el-radio>
            <el-radio :value="2" size="large">{{ t('role.orgAndBelow') }}</el-radio>
            <el-radio :value="4" size="large">{{ t('role.selfOnly') }}</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <div v-show="step !== 1" class="tree">
        <el-tree
          ref="treeRef"
          :default-expanded-keys="form.expandedKeys"
          :data="(menuView as any[]).slice(1)"
          show-checkbox
          node-key="id"
          :props="treeProps"
          @check="nodeClick"
        >
          <template #default="{ data }">
            <span>{{ t(data.title) }}</span>
          </template>
        </el-tree>
      </div>

      <template #footer>
        <span class="dialog-footer">
        <el-button v-blur bg text @click="previousStepAndCancel">
          {{ lastStep ? t('common.previous') : t('common.cancel') }}</el-button
        >
          <el-button v-blur type="primary" :disabled="disabled" @click="nextStepAndSave">
            {{ lastStep ? t('common.save') : t('common.next') }}
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss" scoped>
.role-form {
  padding: 0 2px;

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

    .el-radio-group {
      margin-top: -2px;
      flex-direction: column;
      align-items: flex-start;
    }

    textarea {
      height: 90px;
      resize: none;
    }
  }
}

:deep(.el-dialog__body) {
  position: relative;
  padding-top: 88px;
  height: 540px;
  max-height: 76vh;
}

.el-dialog__body .step {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 56px;
  background: var(--surface-2);
  display: flex;
  align-items: center;
}

.tree {
  width: 438px;
  height: 100%;
  margin: 0 auto;
  overflow-y: auto;
}
</style>
