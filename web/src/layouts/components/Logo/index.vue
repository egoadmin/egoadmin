<script lang="ts" setup name="Logo">
import useSettingsStore from '@/store/modules/settings'

defineProps({
  showLogo: {
    type: Boolean,
    default: true,
  },
  showTitle: {
    type: Boolean,
    default: true,
  },
})

const settingsStore = useSettingsStore()

const title = ref(import.meta.env.VITE_APP_TITLE)

const to = computed(() => {
  const rtn: {
    name?: string
  } = {}
  if (settingsStore.settings.home.enable) {
    rtn.name = 'home'
  }
  return rtn
})
</script>

<template>
  <router-link :to="to" class="title" :class="{ 'is-link': settingsStore.settings.home.enable }" :title="title">
    <svg v-if="showLogo" class="logo-mark" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="1.6">
      <path d="M12 2v20M2 12h20M5 5l14 14M19 5L5 19" stroke-linecap="round" />
    </svg>
    <span v-if="showTitle">{{ title }}</span>
  </router-link>
</template>

<style lang="scss" scoped>
.title {
  position: fixed;
  z-index: 1000;
  top: 0;
  width: inherit;
  padding: 0 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  height: var(--g-sidebar-logo-height);
  text-align: center;
  overflow: hidden;
  text-decoration: none;

  &.is-link {
    cursor: pointer;
  }

  .logo-mark {
    flex: none;

    & + span {
      margin-left: 9px;
    }
  }

  span {
    display: block;
    font-family: var(--font-display);
    font-weight: 680;
    color: var(--fg);

    @include text-overflow;
  }
}
</style>
