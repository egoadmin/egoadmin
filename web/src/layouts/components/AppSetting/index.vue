<script lang="ts" setup name="AppSetting">
import { useClipboard } from '@vueuse/core'
import { ElMessage } from 'element-plus'
import eventBus from '@/utils/eventBus'
import useSettingsStore from '@/store/modules/settings'
import useMenuStore from '@/store/modules/menu'
import { t } from '@/i18n'

const route = useRoute()

const settingsStore = useSettingsStore()
const menuStore = useMenuStore()

const isShow = ref(false)

watch(() => settingsStore.settings.menu.menuMode, (value) => {
  if (value === 'single') {
    menuStore.setActived(0)
  }
  else {
    menuStore.setActived(route.fullPath)
  }
})

onMounted(() => {
  eventBus.on('global-app-setting-toggle', () => {
    isShow.value = !isShow.value
  })
})

const { copy, copied, isSupported } = useClipboard()

watch(copied, (val) => {
  if (val) {
    ElMessage.success(t('appSetting.copySuccess'))
  }
})

function handleCopy() {
  copy(JSON.stringify(settingsStore.settings, null, 2))
}
</script>

<template>
  <div>
    <el-drawer v-model="isShow" :title="t('appSetting.title')" direction="rtl" :size="360">
      <el-alert :title="t('appSetting.notice')" type="error" :closable="false" />
      <el-divider>{{ t('appSetting.colorTheme') }}</el-divider>
      <div class="color-scheme">
        <div class="switch" :class="settingsStore.settings.app.colorScheme" @click="settingsStore.settings.app.colorScheme = settingsStore.settings.app.colorScheme === 'dark' ? 'light' : 'dark'">
          <el-icon class="icon">
            <svg-icon :name="settingsStore.settings.app.colorScheme === 'light' ? 'ep:sunny' : 'ep:moon'" />
          </el-icon>
        </div>
      </div>
      <el-divider v-if="settingsStore.mode === 'pc'">{{ t('appSetting.menuMode') }}</el-divider>
      <div v-if="settingsStore.mode === 'pc'" class="menu-mode">
        <el-tooltip :content="t('appSetting.sideModeWithMain')" placement="top" :show-after="500">
          <div class="mode mode-side" :class="{ active: settingsStore.settings.menu.menuMode === 'side' }" @click="settingsStore.settings.menu.menuMode = 'side'">
            <div class="mode-container" />
            <el-icon>
              <svg-icon name="ep:check" />
            </el-icon>
          </div>
        </el-tooltip>
        <el-tooltip :content="t('appSetting.headMode')" placement="top" :show-after="500">
          <div class="mode mode-head" :class="{ active: settingsStore.settings.menu.menuMode === 'head' }" @click="settingsStore.settings.menu.menuMode = 'head'">
            <div class="mode-container" />
            <el-icon>
              <svg-icon name="ep:check" />
            </el-icon>
          </div>
        </el-tooltip>
        <el-tooltip :content="t('appSetting.sideModeWithoutMain')" placement="top" :show-after="500">
          <div class="mode mode-single" :class="{ active: settingsStore.settings.menu.menuMode === 'single' }" @click="settingsStore.settings.menu.menuMode = 'single'">
            <div class="mode-container" />
            <el-icon>
              <svg-icon name="ep:check" />
            </el-icon>
          </div>
        </el-tooltip>
      </div>
      <el-divider>{{ t('appSetting.navigation') }}</el-divider>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.mainMenuJump') }}
          <el-tooltip :content="t('appSetting.mainMenuJumpTip')" placement="top">
            <el-icon>
              <svg-icon name="ep:question-filled" />
            </el-icon>
          </el-tooltip>
        </div>
        <el-switch v-model="settingsStore.settings.menu.switchMainMenuAndPageJump" :disabled="['single'].includes(settingsStore.settings.menu.menuMode)" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.subMenuUniqueOpened') }}
          <el-tooltip :content="t('appSetting.subMenuUniqueOpenedTip')" placement="top">
            <el-icon>
              <svg-icon name="ep:question-filled" />
            </el-icon>
          </el-tooltip>
        </div>
        <el-switch v-model="settingsStore.settings.menu.subMenuUniqueOpened" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.subMenuCollapse') }}
        </div>
        <el-switch v-model="settingsStore.settings.menu.subMenuCollapse" />
      </div>
      <div v-if="settingsStore.mode === 'pc'" class="setting-item">
        <div class="label">
          {{ t('appSetting.subMenuCollapseButton') }}
        </div>
        <el-switch v-model="settingsStore.settings.menu.enableSubMenuCollapseButton" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.enableHotkeys') }}
        </div>
        <el-switch v-model="settingsStore.settings.menu.enableHotkeys" :disabled="['single'].includes(settingsStore.settings.menu.menuMode)" />
      </div>
      <el-divider>{{ t('appSetting.topbar') }}</el-divider>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.mode') }}
        </div>
        <el-radio-group v-model="settingsStore.settings.topbar.mode" size="small">
          <el-radio-button value="static">
            {{ t('appSetting.static') }}
          </el-radio-button>
          <el-radio-button value="fixed">
            {{ t('appSetting.fixed') }}
          </el-radio-button>
          <el-radio-button value="sticky">
            {{ t('appSetting.sticky') }}
          </el-radio-button>
        </el-radio-group>
      </div>
      <el-divider>{{ t('appSetting.toolbar') }}</el-divider>
      <div v-if="settingsStore.mode === 'pc'" class="setting-item">
        <div class="label">
          {{ t('appSetting.fullscreen') }}
          <el-tooltip :content="t('appSetting.fullscreenTip')" placement="top">
            <el-icon>
              <svg-icon name="ep:question-filled" />
            </el-icon>
          </el-tooltip>
        </div>
        <el-switch v-model="settingsStore.settings.toolbar.enableFullscreen" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.pageReload') }}
          <el-tooltip :content="t('appSetting.pageReloadTip')" placement="top">
            <el-icon>
              <svg-icon name="ep:question-filled" />
            </el-icon>
          </el-tooltip>
        </div>
        <el-switch v-model="settingsStore.settings.toolbar.enablePageReload" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.colorScheme') }}
          <el-tooltip :content="t('appSetting.colorSchemeTip')" placement="top">
            <el-icon>
              <svg-icon name="ep:question-filled" />
            </el-icon>
          </el-tooltip>
        </div>
        <el-switch v-model="settingsStore.settings.toolbar.enableColorScheme" />
      </div>
      <el-divider v-if="settingsStore.mode === 'pc'">{{ t('appSetting.breadcrumb') }}</el-divider>
      <div v-if="settingsStore.mode === 'pc'" class="setting-item">
        <div class="label">
          {{ t('appSetting.enabled') }}
        </div>
        <el-switch v-model="settingsStore.settings.breadcrumb.enable" />
      </div>
      <el-divider>{{ t('appSetting.navSearch') }}</el-divider>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.enabled') }}
          <el-tooltip :content="t('appSetting.navSearchTip')" placement="top">
            <el-icon>
              <svg-icon name="ep:question-filled" />
            </el-icon>
          </el-tooltip>
        </div>
        <el-switch v-model="settingsStore.settings.navSearch.enable" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.enableHotkeys') }}
        </div>
        <el-switch v-model="settingsStore.settings.navSearch.enableHotkeys" :disabled="!settingsStore.settings.navSearch.enable" />
      </div>
      <el-divider>{{ t('appSetting.copyright') }}</el-divider>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.enabled') }}
        </div>
        <el-switch v-model="settingsStore.settings.copyright.enable" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.dates') }}
        </div>
        <el-input v-model="settingsStore.settings.copyright.dates" size="small" :disabled="!settingsStore.settings.copyright.enable" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.company') }}
        </div>
        <el-input v-model="settingsStore.settings.copyright.company" size="small" :disabled="!settingsStore.settings.copyright.enable" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.website') }}
        </div>
        <el-input v-model="settingsStore.settings.copyright.website" size="small" :disabled="!settingsStore.settings.copyright.enable" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.beian') }}
        </div>
        <el-input v-model="settingsStore.settings.copyright.beian" size="small" :disabled="!settingsStore.settings.copyright.enable" />
      </div>
      <el-divider>{{ t('appSetting.home') }}</el-divider>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.homeEnable') }}
          <el-tooltip :content="t('appSetting.homeEnableTip')" placement="top">
            <el-icon>
              <svg-icon name="ep:question-filled" />
            </el-icon>
          </el-tooltip>
        </div>
        <el-switch v-model="settingsStore.settings.home.enable" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.homeTitle') }}
        </div>
        <el-input v-model="settingsStore.settings.home.title" size="small" />
      </div>
      <el-divider>{{ t('appSetting.other') }}</el-divider>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.elementSize') }}
          <el-tooltip :content="t('appSetting.elementSizeTip')" placement="top">
            <el-icon>
              <svg-icon name="ep:question-filled" />
            </el-icon>
          </el-tooltip>
        </div>
        <el-radio-group v-model="settingsStore.settings.app.elementSize" size="small">
          <el-radio-button value="large">
            {{ t('appSetting.large') }}
          </el-radio-button>
          <el-radio-button value="default">
            {{ t('appSetting.default') }}
          </el-radio-button>
          <el-radio-button value="small">
            {{ t('appSetting.small') }}
          </el-radio-button>
        </el-radio-group>
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.enablePermission') }}
        </div>
        <el-switch v-model="settingsStore.settings.app.enablePermission" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.progress') }}
          <el-tooltip :content="t('appSetting.progressTip')" placement="top">
            <el-icon>
              <svg-icon name="ep:question-filled" />
            </el-icon>
          </el-tooltip>
        </div>
        <el-switch v-model="settingsStore.settings.app.enableProgress" />
      </div>
      <div class="setting-item">
        <div class="label">
          {{ t('appSetting.dynamicTitle') }}
          <el-tooltip :content="t('appSetting.dynamicTitleTip')" placement="top">
            <el-icon>
              <svg-icon name="ep:question-filled" />
            </el-icon>
          </el-tooltip>
        </div>
        <el-switch v-model="settingsStore.settings.app.enableDynamicTitle" />
      </div>
      <template v-if="isSupported" #footer>
        <el-button type="primary" @click="handleCopy">
          <template #icon>
            <el-icon>
              <svg-icon name="ep:document-copy" />
            </el-icon>
          </template>
          {{ t('appSetting.copyConfig') }}
        </el-button>
      </template>
    </el-drawer>
  </div>
</template>

<style lang="scss" scoped>
:deep(.el-drawer__header) {
  margin-bottom: initial;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--el-border-color);
}

:deep(.el-drawer__footer) {
  padding: 20px;
  border-top: 1px solid var(--el-border-color);
  transition: var(--el-transition-border);

  .el-button {
    width: 100%;
  }
}

:deep(.el-divider) {
  margin: 36px 0 24px;
}

.color-scheme {
  display: flex;
  align-items: center;
  justify-content: center;
  padding-bottom: 10px;

  $width: 50px;

  .switch {
    width: $width;
    height: 30px;
    border-radius: 15px;
    cursor: pointer;
    background-color: var(--el-fill-color-darker);
    transition: background-color 0.3s;

    &.dark {
      .icon {
        margin-left: calc($width - 24px - 3px);
      }
    }

    .icon {
      margin: 3px;
      padding: 5px;
      font-size: 24px;
      border-radius: 50%;
      background-color: var(--el-fill-color-lighter);
      transition: margin-left 0.3s, background-color 0.3s;
    }
  }
}

.menu-mode {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  padding-bottom: 10px;

  .mode {
    position: relative;
    width: 80px;
    height: 55px;
    margin: 10px;
    border-radius: 5px;
    overflow: hidden;
    cursor: pointer;
    background-color: var(--g-app-bg);
    box-shadow: 0 0 5px 1px var(--el-border-color-lighter);
    transition: 0.2s;

    &:hover {
      box-shadow: 0 0 5px 1px var(--el-border-color-darker);
    }

    &.active {
      box-shadow: 0 0 0 2px var(--el-color-primary);
    }

    &::before,
    &::after,
    .mode-container {
      pointer-events: none;
      position: absolute;
      border-radius: 3px;
    }

    .mode-container::before {
      content: "";
      position: absolute;
      width: 100%;
      height: 100%;
      background-color: var(--g-sub-sidebar-menu-active-bg);
      opacity: 0.2;
    }

    &-side {
      &::before {
        content: "";
        top: 5px;
        left: 5px;
        bottom: 5px;
        width: 10px;
        background-color: var(--g-sub-sidebar-menu-active-bg);
      }

      &::after {
        content: "";
        top: 5px;
        left: 20px;
        bottom: 5px;
        width: 15px;
        background-color: var(--g-sub-sidebar-menu-active-bg);
        opacity: 0.5;
      }

      .mode-container {
        inset: 5px 5px 5px 40px;
        border: 1px dashed var(--g-sub-sidebar-menu-active-bg);
      }
    }

    &-head {
      &::before {
        content: "";
        top: 5px;
        left: 5px;
        right: 5px;
        height: 10px;
        background-color: var(--g-sub-sidebar-menu-active-bg);
      }

      &::after {
        content: "";
        top: 20px;
        left: 5px;
        bottom: 5px;
        width: 15px;
        background-color: var(--g-sub-sidebar-menu-active-bg);
        opacity: 0.5;
      }

      .mode-container {
        inset: 20px 5px 5px 25px;
        border: 1px dashed var(--g-sub-sidebar-menu-active-bg);
      }
    }

    &-single {
      &::before {
        content: "";
        position: absolute;
        top: 5px;
        left: 5px;
        bottom: 5px;
        width: 15px;
        background-color: var(--g-sub-sidebar-menu-active-bg);
        opacity: 0.5;
      }

      .mode-container {
        inset: 5px 5px 5px 25px;
        border: 1px dashed var(--g-sub-sidebar-menu-active-bg);
      }
    }

    i {
      position: absolute;
      right: 10px;
      bottom: 10px;
      display: none;
    }

    &.active i {
      display: block;
      color: var(--el-color-primary);
    }
  }
}

.setting-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 5px 0;
  padding: 5px 10px;
  border-radius: 5px;
  transition: all 0.3s;

  &:hover {
    background: var(--el-fill-color);
  }

  .label {
    font-size: 14px;
    color: var(--el-text-color-regular);
    display: flex;
    align-items: center;

    i {
      margin-left: 4px;
      font-size: 17px;
      color: var(--el-color-warning);
      cursor: help;
    }
  }

  .el-switch {
    height: auto;
  }

  .el-input {
    width: 150px;
  }
}
</style>
