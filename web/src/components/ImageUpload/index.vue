<script lang="ts" setup name="ImageUpload">
import type { UploadProps } from 'element-plus'
import { ElMessage } from 'element-plus'
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
    default: 'image',
  },
  name: {
    type: String,
    default: 'file',
  },
  url: {
    type: String,
    default: '',
  },
  size: {
    type: Number,
    default: 2,
  },
  width: {
    type: Number,
    default: 150,
  },
  height: {
    type: Number,
    default: 150,
  },
  placeholder: {
    type: String,
    default: '',
  },
  notip: {
    type: Boolean,
    default: false,
  },
  ext: {
    type: Array,
    default: () => ['jpg', 'png', 'gif', 'bmp'],
  },
})

const emit = defineEmits(['update:url', 'onSuccess'])

const uploadData = ref({
  imageViewerVisible: false,
  progress: {
    preview: '',
    percent: 0,
  },
})

// 预览
function preview() {
  uploadData.value.imageViewerVisible = true
}
// 关闭预览
function previewClose() {
  uploadData.value.imageViewerVisible = false
}
// 移除
function remove() {
  emit('update:url', '')
}
const beforeUpload: UploadProps['beforeUpload'] = (file) => {
  const fileName = file.name.split('.')
  const fileExt = fileName.at(-1)
  const isTypeOk = props.ext.includes(fileExt)
  const isSizeOk = file.size / 1024 / 1024 < props.size
  if (!isTypeOk) {
    ElMessage.error(t('upload.imageTypeError', { ext: props.ext.join(' / ') }))
  }
  if (!isSizeOk) {
    ElMessage.error(t('upload.imageSizeError', { size: props.size }))
  }
  if (isTypeOk && isSizeOk) {
    uploadData.value.progress.preview = URL.createObjectURL(file)
  }
  return isTypeOk && isSizeOk
}
const onProgress: UploadProps['onProgress'] = (file) => {
  uploadData.value.progress.percent = ~~file.percent
}
const onSuccess: UploadProps['onSuccess'] = (res) => {
  uploadData.value.progress.preview = ''
  uploadData.value.progress.percent = 0
  const uploadedUrl = (res as { url?: string })?.url ?? ''
  if (uploadedUrl) {
    emit('update:url', uploadedUrl)
  }
  emit('onSuccess', res)
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
  <div class="upload-container">
    <el-upload
      :show-file-list="false"
      :headers="headers"
      :action="action"
      :data="data"
      :name="name"
      :http-request="uploadRequest"
      :before-upload="beforeUpload"
      :on-progress="onProgress"
      :on-success="onSuccess"
      drag
      class="image-upload"
    >
      <el-image v-if="url === ''" :src="url === '' ? placeholder : url" :style="`width:${width}px;height:${height}px;`" fit="fill">
        <template #error>
          <div class="image-slot" :style="`width:${width}px;height:${height}px;`">
            <el-icon>
              <svg-icon name="ep:plus" />
            </el-icon>
          </div>
        </template>
      </el-image>
      <div v-else class="image">
        <el-image :src="url" :style="`width:${width}px;height:${height}px;`" fit="fill" />
        <div class="mask">
          <div class="actions">
            <span :title="t('upload.preview')" @click.stop="preview">
              <el-icon>
                <svg-icon name="ep:zoom-in" />
              </el-icon>
            </span>
            <span :title="t('upload.remove')" @click.stop="remove">
              <el-icon>
                <svg-icon name="ep:delete" />
              </el-icon>
            </span>
          </div>
        </div>
      </div>
      <div v-show="url === '' && uploadData.progress.percent" class="progress" :style="`width:${width}px;height:${height}px;`">
        <el-image :src="uploadData.progress.preview" :style="`width:${width}px;height:${height}px;`" fit="fill" />
        <el-progress type="circle" :width="Math.min(width, height) * 0.8" :percentage="uploadData.progress.percent" />
      </div>
    </el-upload>
    <div v-if="!notip" class="el-upload__tip">
      <div style="display: inline-block;">
        <el-alert :title="t('upload.imageTip', { ext: ext.join(' / '), size, width, height })" type="info" show-icon :closable="false" />
      </div>
    </div>
    <el-image-viewer v-if="uploadData.imageViewerVisible" :url-list="[url]" teleported @close="previewClose" />
  </div>
</template>

<style lang="scss" scoped>
.upload-container {
  line-height: initial;
}

.el-image {
  display: block;
}

.image {
  position: relative;
  border-radius: 6px;
  overflow: hidden;

  .mask {
    opacity: 0;
    position: absolute;
    top: 0;
    width: 100%;
    height: 100%;
    background-color: var(--el-overlay-color-lighter);
    transition: opacity 0.3s;

    .actions {
      width: 100px;
      height: 100px;
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: center;

      @include position-center(xy);

      span {
        width: 50%;
        text-align: center;
        cursor: pointer;
        color: var(--el-color-white);
        transition: color 0.1s, transform 0.1s;

        &:hover {
          transform: scale(1.5);
        }

        .el-icon {
          font-size: 24px;
        }
      }
    }
  }

  &:hover .mask {
    opacity: 1;
  }
}

.image-upload {
  display: inline-block;
  vertical-align: top;
}

:deep(.el-upload) {
  .el-upload-dragger {
    display: inline-block;
    padding: 0;

    &.is-dragover {
      border-width: 1px;
    }

    .image-slot {
      display: flex;
      justify-content: center;
      align-items: center;
      width: 100%;
      height: 100%;
      color: var(--el-text-color-placeholder);
      background-color: transparent;

      i {
        font-size: 30px;
      }
    }

    .progress {
      position: absolute;
      top: 0;

      &::after {
        content: "";
        position: absolute;
        width: 100%;
        height: 100%;
        left: 0;
        top: 0;
        background-color: var(--el-overlay-color-lighter);
      }

      .el-progress {
        z-index: 1;

        @include position-center(xy);

        .el-progress__text {
          color: var(--el-text-color-placeholder);
        }
      }
    }
  }
}
</style>
