<script lang="ts" setup name="SubSidebar">
import Logo from '../Logo/index.vue'
import SidebarItem from '../SidebarItem/index.vue'
import useSettingsStore from '@/store/modules/settings'
import useMenuStore from '@/store/modules/menu'

const settingsStore = useSettingsStore()
const menuStore = useMenuStore()

const sidebarScrollTop = ref(0)

function onSidebarScroll(e: Event) {
  sidebarScrollTop.value = (e.target as HTMLElement).scrollTop
}
</script>

<template>
  <div
    v-if="['side', 'head', 'single'].includes(settingsStore.settings.menu.menuMode) || settingsStore.mode === 'mobile'"
    class="sub-sidebar-container"
    :class="{ 'is-collapse': settingsStore.mode === 'pc' && settingsStore.settings.menu.subMenuCollapse }"
    @scroll="onSidebarScroll"
  >
    <Logo
      :show-logo="settingsStore.settings.menu.menuMode === 'single'" class="sidebar-logo" :class="{
        'sidebar-logo-bg': settingsStore.settings.menu.menuMode === 'single',
        'shadow': sidebarScrollTop,
      }"
    />
    <transition-group name="sub-sidebar">
      <template v-for="(mainItem, mainIndex) in menuStore.allMenus" :key="mainIndex">
        <div v-show="mainIndex === menuStore.actived">
          <el-menu
            :unique-opened="settingsStore.settings.menu.subMenuUniqueOpened"
            :default-openeds="menuStore.defaultOpenedPaths" :default-active="$route.meta.activeMenu || $route.path"
            :collapse="settingsStore.mode === 'pc' && settingsStore.settings.menu.subMenuCollapse"
            :collapse-transition="false" :class="{
              'is-collapse-without-logo': settingsStore.settings.menu.menuMode !== 'single' && settingsStore.settings.menu.subMenuCollapse,
            }"
          >
            <template v-for="(route, index) in mainItem.children">
              <SidebarItem
                v-if="route.meta?.sidebar !== false" :key="route.path || index" :item="route"
                :base-path="route.path" popper-class="subsidebar-popper"
              />
            </template>
          </el-menu>
        </div>
      </template>
    </transition-group>

    <div
      v-if="settingsStore.settings.menu.enableSubMenuCollapseButton" class="sidebar-collapse"
      :class="{ 'is-collapse': settingsStore.settings.menu.subMenuCollapse }"
      @click="settingsStore.toggleSidebarCollapse()"
    >
      <img src="@/assets/icons/collapse.svg" alt="">
    </div>
  </div>
</template>

<style lang="scss" scoped>
.sub-sidebar-container {
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;

  // firefox隐藏滚动条
  scrollbar-width: none;

  // chrome隐藏滚动条
  &::-webkit-scrollbar {
    display: none;
  }

  width: var(--g-sub-sidebar-width);
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  padding: 16px 8px 0;
  background-color: var(--g-sub-sidebar-bg);
  box-shadow: 10px 0 10px -10px var(--g-box-shadow-color);
  transition:
    background-color 0.3s,
    var(--el-transition-box-shadow),
    left 0.3s,
    width 0.3s;
  border-top: 1px solid #e7e7e7;

  &.is-collapse {
    width: 64px;

    .sidebar-logo {
      &:not(.sidebar-logo-bg) {
        display: none;
      }

      :deep(span) {
        display: none;
      }
    }
  }

  .sidebar-logo {
    transition: box-shadow 0.2s, background-color 0.3s, color 0.3s;
    background-color: var(--g-sub-sidebar-bg);

    &:not(.sidebar-logo-bg) {
      :deep(span) {
        color: var(--g-sub-sidebar-menu-color);
      }
    }

    &.sidebar-logo-bg {
      background-color: var(--g-main-sidebar-bg);
    }

    &.shadow {
      box-shadow: 0 10px 10px -10px var(--g-box-shadow-color);
    }
  }

  .el-menu {
    border-right: 0;
    padding-top: var(--g-sidebar-logo-height);
    transition: border-color 0.3s, background-color 0.3s, color 0.3s, padding-top 0.3s;
    background-color: var(--g-sub-sidebar-bg);

    :deep(.el-menu-item) {
      border-radius: 3px;
      height: 36px;

      .title {
        margin-left: 15px;
      }

      .el-menu--inline {
        margin-bottom: 4px;
      }
    }

    :deep(.el-menu-item.is-active) {
      position: relative;
      border-radius: 0 3px 3px 0;

      &::after {
        position: absolute;
        top: 0;
        left: 0;
        content: "";
        display: block;
        width: 2px;
        height: 100%;
        background: var(--g-sub-sidebar-menu-active-color);
      }
    }

    &:not(.el-menu--collapse) {
      width: inherit;
    }

    &.is-collapse-without-logo {
      padding-top: 0;
    }

    &.el-menu--collapse {
      :deep(.title-icon) {
        margin-right: 0;
      }

      :deep(.el-menu-item),
      :deep(.el-sub-menu__title) {
        span,
        .el-sub-menu__icon-arrow {
          display: none;
        }
      }
    }

    &.menu-radius:not(.el-menu--collapse) {
      .sidebar-item {
        padding: 0 10px;

        &:first-child {
          padding-top: 10px;
        }

        &:last-child {
          padding-bottom: 10px;
        }
      }

      :deep(.el-menu--inline),
      :deep(.el-menu-item),
      :deep(.el-sub-menu__title) {
        border-radius: 10px;
      }
    }

    :deep(.el-sub-menu) {
      .el-sub-menu__title {
        height: 36px;
        margin-bottom: 4px;
      }

      .sidebar-item {
        margin-bottom: 4px;
      }
    }
  }

  .sidebar-collapse {
    width: 20px;
    height: 20px;
    position: absolute;
    left: 22px;
    bottom: 13px;
    cursor: pointer;
    overflow: hidden;

    img {
      width: 100%;
    }

    &:hover img {
      content: url("@/assets/icons/collapse-hover.svg");
    }
  }
}

// 次侧边栏动画
.sub-sidebar-enter-active {
  transition: opacity 0.3s, transform 0.3s;
}

.sub-sidebar-enter-from,
.sub-sidebar-leave-active {
  opacity: 0;
  transform: translateY(30px) skewY(10deg);
}

.sub-sidebar-leave-active {
  position: absolute;
}

// 菜单收起样式
.el-menu--collapse {
  :deep(.el-sub-menu__title) {
    padding: 0;
    padding: 0 14px;
    width: 48px;
    height: 36px;
    margin-bottom: 4px;
    border-radius: 3px;
  }

  :deep(.is-active > .el-sub-menu__title) {
    border-radius: 0 3px 3px 0;
    position: relative;

    &::after {
      content: "";
      position: absolute;
      top: 0;
      left: 0;
      width: 2px;
      height: 100%;
      background: var(--g-sub-sidebar-menu-active-color);
    }
  }
}
</style>

<style lang="scss">
.subsidebar-popper {
  .el-menu {
    min-width: 102px;

    .sidebar-item {
      .el-menu-item {
        height: 36px;
      }

      .title {
        text-align: center;
      }
    }
  }
}
</style>
