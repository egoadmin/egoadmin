<script lang="ts" setup name="FileUpload">
import { ElMessage } from 'element-plus'
import type { UploadProps, UploadUserFile } from 'element-plus'
import { t } from '@/i18n'
import apiUpload from '@/api/modules/upload'

const props = defineProps({
  action: {
    type: String,
    required: true,
  },
  headers: {
    type: Object,
    default: () => {},
  },
  data: {
    type: Object,
    default: () => {},
  },
  profile: {
    type: String,
    default: 'document',
  },
  name: {
    type: String,
    default: 'file',
  },
  size: {
    type: Number,
    default: 2,
  },
  max: {
    type: Number,
    default: 3,
  },
  files: {
    type: Array,
    default: () => [],
  },
  notip: {
    type: Boolean,
    default: false,
  },
  ext: {
    type: Array,
    default: () => ['zip', 'rar'],
  },
})

const emit = defineEmits(['onSuccess'])

const beforeUpload: UploadProps['beforeUpload'] = (file) => {
  const fileName = file.name.split('.')
  const fileExt = fileName.at(-1)
  const isTypeOk = props.ext.includes(fileExt)
  const isSizeOk = file.size / 1024 / 1024 < props.size
  if (!isTypeOk) {
    ElMessage.error(t('upload.fileTypeError', { ext: props.ext.join(' / ') }))
  }
  if (!isSizeOk) {
    ElMessage.error(t('upload.fileSizeError', { size: props.size }))
  }
  return isTypeOk && isSizeOk
}

const onExceed: UploadProps['onExceed'] = () => {
  ElMessage.warning(t('upload.fileExceed'))
}

const onSuccess: UploadProps['onSuccess'] = (res, file, fileList) => {
  emit('onSuccess', res, file, fileList)
}

const uploadRequest: UploadProps['httpRequest'] = async (options) => {
  const file = options.file as File
  try {
    const res = await apiUpload.uploadFile({
      file,
      profile: props.profile,
      onProgress: percent => options.onProgress({ percent } as any),
    })
    options.onSuccess(res)
  } catch (err) {
    options.onError(err as any)
  }
}
</script>

<template>
  <el-upload
    :headers="headers"
    :action="action"
    :data="data"
    :name="name"
    :http-request="uploadRequest"
    :before-upload="beforeUpload"
    :on-exceed="onExceed"
    :on-success="onSuccess"
    :file-list="files as UploadUserFile[]"
    :limit="max"
    drag
  >
    <div class="slot">
      <el-icon class="el-icon--upload">
        <svg-icon name="ep:upload-filled" />
      </el-icon>
      <div class="el-upload__text">
        {{ t('upload.fileDragPrefix') }}<em>{{ t('upload.clickUpload') }}</em>
      </div>
    </div>
    <template #tip>
      <div v-if="!notip" class="el-upload__tip">
        <div style="display: inline-block;">
          <el-alert :title="t('upload.fileTip', { ext: ext.join(' / '), size, max })" type="info" show-icon :closable="false" />
        </div>
      </div>
    </template>
  </el-upload>
</template>

<style lang="scss" scoped>
:deep(.el-upload.is-drag) {
  display: inline-block;

  .el-upload-dragger {
    padding: 0;
  }

  &.is-dragover {
    border-width: 1px;
  }

  .slot {
    width: 300px;
    padding: 40px 0;
  }
}
</style>
