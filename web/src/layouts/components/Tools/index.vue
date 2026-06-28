<script lang="ts" setup name="Tools">
import eventBus from '@/utils/eventBus'
import LanguageSwitcher from '@/components/LanguageSwitcher/index.vue'
import useSettingsStore from '@/store/modules/settings'
import useUserStore from '@/store/modules/user'
import { avatarUrl } from '@/utils'

const router = useRouter()

const settingsStore = useSettingsStore()
const userStore = useUserStore()
const headerAvatarUrl = computed(() => avatarUrl(userStore.userInfo))

const isDark = computed(() => settingsStore.settings.app.colorScheme === 'dark')
function toggleTheme() {
  settingsStore.setColorScheme(isDark.value ? 'light' : 'dark')
}

const hintDialogRef = ref()
function logOut() {
  hintDialogRef.value.dialogVisibleChange(false)
  userStore.logout()
}

function userCommand(command: 'setting' | 'logout') {
  switch (command) {
    case 'setting':
      router.push({
        name: 'personalSetting',
      })
      break
    case 'logout':
      hintDialogRef.value.dialogVisibleChange(true)
      break
  }
}
</script>

<template>
  <div class="tools">
    <div class="buttons">
      <button class="icon-btn" :title="$t('app.search')" @click="eventBus.emit('global-search-toggle')">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <circle cx="11" cy="11" r="7" /><path d="M21 21l-4.3-4.3" stroke-linecap="round" />
        </svg>
      </button>
      <button class="icon-btn" :title="$t('app.toggleTheme')" @click="toggleTheme">
        <svg v-if="!isDark" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
          <path d="M12 3a9 9 0 1 0 9 9c-5 0-9-4-9-9z" />
        </svg>
        <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
          <circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.5 1.5M17.5 17.5L19 19M19 5l-1.5 1.5M6.5 17.5L5 19" stroke-linecap="round" />
        </svg>
      </button>
      <LanguageSwitcher />
    </div>
    <el-dropdown class="user-container" size="default" popper-class="header-avatar-popper" @command="userCommand">
      <div class="user-wrapper">
        <span class="user-pic">
          <img v-if="headerAvatarUrl" :src="headerAvatarUrl" alt="">
          <template v-else>{{ (userStore.userInfo?.username || 'U').charAt(0).toUpperCase() }}</template>
        </span>
        <span class="user-name">{{ userStore.userInfo?.username }}</span>
        <el-icon class="user-caret">
          <svg-icon name="ep:caret-bottom" />
        </el-icon>
      </div>
      <template #dropdown>
        <el-dropdown-menu class="user-dropdown">
          <el-dropdown-item command="setting">{{ $t('app.personalCenter') }}</el-dropdown-item>
          <el-dropdown-item command="logout">{{ $t('app.logout') }}</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
    <hint-dialog ref="hintDialogRef" :title="$t('app.tip')" icon-class-name="icon-warning" :confirm="logOut" :cancel="() => hintDialogRef.dialogVisibleChange(false)" :content="$t('layout.quitSystem')" />
  </div>
</template>

<style lang="scss" scoped>
.tools {
  display: flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;

  .buttons {
    display: flex;
    align-items: center;
    gap: 8px;
  }
}

.icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  color: var(--muted);
  cursor: pointer;
  background: none;
  border: 0;
  border-radius: var(--r-md);
  transition: background-color 0.15s var(--ease), color 0.15s var(--ease);

  &:hover {
    background-color: var(--surface-2);
    color: var(--fg);
  }
}

:deep(.user-container) {
  display: inline-flex;
  align-items: center;
  height: 64px;
  cursor: pointer;

  .user-wrapper {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 8px 5px 6px;
    outline: none;
    border-radius: var(--r-pill);
    transition: background-color 0.15s var(--ease);

    &:hover {
      background-color: var(--surface-2);
    }

    .user-pic {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      overflow: hidden;
      font-size: 12px;
      font-weight: 600;
      color: #fff;
      background: var(--accent);
      border-radius: 50%;

      img {
        width: 100%;
        height: 100%;
        object-fit: cover;
      }
    }

    .user-name {
      font-size: 13.5px;
      font-weight: 550;
      color: var(--fg);
    }

    .user-caret {
      color: var(--muted);
      font-size: 14px;
    }
  }
}
</style>

<style lang="scss">
// 头像下拉与顶部分组下拉风格一致（设计 .grp-drop）
.header-avatar-popper {
  min-width: 152px;

  .el-dropdown-menu__item {
    padding: 8px 12px;
    font-size: 13.5px;
    font-weight: 500;
    color: var(--text);
    border-radius: var(--r-sm);

    &:hover {
      background-color: var(--surface-2);
      color: var(--fg);
    }
  }

  .el-popper__arrow {
    visibility: hidden;
  }
}
</style>
