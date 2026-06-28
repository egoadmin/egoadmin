<script lang="ts" setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import apiUpload from '@/api/modules/upload'
import { t } from '@/i18n'

const emits = defineEmits(['editAvatar'])

const cropperRef = ref()
const cropperOption = ref({
  img: '', // 裁剪图片的地址
  outputSize: 1,
  outputType: 'png', // 裁剪生成图片的格式
  autoCrop: true, // 默认生成截图框
  fixedBox: true, // 固定截图框大小
  canMoveBox: true, // 截图框可以拖动
  autoCropWidth: 120, // 截图框宽度
  autoCropHeight: 120, // 截图框高度
  fixed: true, // 截图框宽高固定比例
  fixedNumber: [1, 1], // 截图框的宽高比例
  centerBox: true, // 截图框被限制在图片里面
  canMove: true, // 上传图片不允许拖动
  canScale: true, // 上传图片不允许滚轮缩放
})

const currentImg = ref({
  url: '',
})
const dialogImgVisible = ref(false)
function dialogImgVisibleChange(val: boolean) {
  dialogImgVisible.value = val
}

function close() {
  cropperOption.value.img = ''
  currentImg.value = {
    url: '',
  }
}

// 图片预览
function realTime(data: any) {
  currentImg.value = { url: data?.url ?? '' }
}

function uploadChange(file: any) {
  console.log(file)
  const { raw, name } = file
  const suffix = name.slice(name.lastIndexOf('.'), name.length)
  if (!['.jpg', '.png', '.jpeg'].includes(suffix.toLowerCase())) {
    ElMessage.warning(t('personal.avatarFormatError'))
    return
  }

  cropperOption.value.img = URL.createObjectURL(raw)
}

async function confirm() {
  const { url } = currentImg.value
  if (!url) {
    return
  }
  const blob = await new Promise<Blob>((resolve) => {
    cropperRef.value.getCropBlob((data: Blob) => resolve(data))
  })
  const file = new File([blob], 'avatar.png', { type: blob.type || 'image/png' })
  const uploaded = await apiUpload.uploadFile({
    file,
    profile: 'avatar',
  })
  emits('editAvatar', { referenceId: uploaded.referenceId })
}

defineExpose({
  dialogImgVisibleChange,
})
</script>

<template>
  <div>
    <el-dialog
      v-model="dialogImgVisible"
      :close-on-click-modal="false"
      :title="t('personal.uploadAvatar')"
      align-center
      width="440px"
      @close="close"
    >
      <div class="head-portrait">
        <div v-if="cropperOption.img">
          <div style="width: 200px; height: 200px">
            <VueCropper
              ref="cropperRef"
              :output-size="cropperOption.outputSize"
              :output-type="cropperOption.outputType"
              :img="cropperOption.img"
              :auto-crop="cropperOption.autoCrop"
              :fixed-box="cropperOption.fixedBox"
              :can-move-box="cropperOption.canMoveBox"
              :auto-crop-width="cropperOption.autoCropWidth"
              :auto-crop-height="cropperOption.autoCropHeight"
              :center-box="cropperOption.centerBox"
              :fixed="cropperOption.fixed"
              :fixed-number="cropperOption.fixedNumber"
              :can-move="cropperOption.canMove"
              :can-scale="cropperOption.canScale"
              @real-time="realTime"
            />
          </div>
          <el-upload :auto-upload="false" :show-file-list="false" :on-change="uploadChange">
            <div class="upload-text">{{ t('personal.reupload') }}</div>
          </el-upload>
        </div>
        <div v-else>
          <el-upload :auto-upload="false" :show-file-list="false" :on-change="uploadChange">
            <div class="no-upload">
              <el-icon style="font-size: 20px">
                <svg-icon name="add-avatar" />
              </el-icon>
              <div>{{ t('personal.addImage') }}</div>
              <div style="width: 158px; text-align: center">
                {{ t('personal.avatarUploadTip') }}
              </div>
            </div>
          </el-upload>
        </div>
        <div class="avatar-preview">
          <div class="current-img">
            <img v-if="currentImg.url" :src="currentImg.url" alt="" />
          </div>
          <div class="text">{{ t('personal.avatarPreview') }}</div>
        </div>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button v-blur bg text @click="dialogImgVisible = false">{{ t('common.cancel') }}</el-button>
          <el-button v-blur type="primary" @click="confirm">{{ t('common.save') }}</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss" scoped>
.head-portrait {
  display: flex;

  .no-upload {
    width: 200px;
    height: 200px;
    color: var(--faint);
    font-size: 12px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background: var(--surface-2);

    & > div:nth-child(2) {
      color: var(--fg);
      margin: 6px 0 16px;
    }
  }

  .upload-text {
    color: var(--el-color-primary);
    margin-top: 8px;

    &:hover {
      color: var(--accent);
    }
  }

  .avatar-preview {
    margin-left: auto;
    display: flex;
    flex-direction: column;
    align-items: center;

    .current-img {
      width: 120px;
      height: 120px;
      overflow: hidden;
      border-radius: 50%;
      background: var(--surface-2);

      img,
      :deep(.show-preview) {
        width: 120px !important;
        height: 120px !important;
      }

      img {
        display: block;
        object-fit: cover;
      }
    }

    .text {
      margin-top: 12px;
    }
  }
}
</style>
