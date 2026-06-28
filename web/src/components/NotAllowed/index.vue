<script lang="ts" setup>
const router = useRouter()

const data = ref({
  inter: NaN,
  countdown: 5,
})

onUnmounted(() => {
  data.value.inter && window.clearInterval(data.value.inter)
})

onMounted(() => {
  data.value.inter = window.setInterval(() => {
    data.value.countdown--
    if (data.value.countdown === 0) {
      data.value.inter && window.clearInterval(data.value.inter)
      goBack()
    }
  }, 1000)
})

function goBack() {
  router.push('/')
}
</script>

<template>
  <div class="notallowed">
    <svg-icon name="403" class="icon" />
    <div class="content">
      <h1>403</h1>
      <div class="desc">
        {{ $t('app.notAllowed') }}
      </div>
      <el-button v-blur type="primary" @click="goBack">
        {{ $t('app.backHomeIn', { countdown: data.countdown }) }}
      </el-button>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.notallowed {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 700px;

  @include position-center(xy);

  .icon {
    width: 400px;
    height: 400px;
  }

  .content {
    h1 {
      margin: 0;
      font-size: 72px;
      color: var(--el-text-color-primary);
    }

    .desc {
      margin: 20px 0 30px;
      font-size: 20px;
      color: var(--el-text-color-secondary);
    }
  }
}
</style>
