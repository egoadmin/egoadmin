<script lang="ts" setup>
const props = defineProps({
  // 图标名称
  name: {
    type: String,
    default: '',
  },
})

const name = computed(() => props.name)
</script>

<template>
  <svg id="icon" aria-hidden="true">
    <use
      :xlink:href="name"
    />
  </svg>
</template>

<style lang="scss" scoped>
#icon {
  width: 1em;
  height: 1em;
  vertical-align: -0.15em;
  fill: currentcolor;
  overflow: hidden;
}
</style>
