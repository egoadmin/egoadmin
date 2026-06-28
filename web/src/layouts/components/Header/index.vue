<script lang="ts" setup name="Header">
import Tools from '../Tools/index.vue'
import { resolveRoutePath } from '@/utils'
import { resolveMenuTitle } from '@/utils/i18n'
import useSettingsStore from '@/store/modules/settings'
import useMenuStore from '@/store/modules/menu'
import type { Menu } from '#/global'

const settingsStore = useSettingsStore()
const menuStore = useMenuStore()
const route = useRoute()
const router = useRouter()

const title = ref(import.meta.env.VITE_APP_TITLE)

const brandTo = computed(() => (settingsStore.settings.home.enable ? { name: 'home' } : {}))

// 顶部分组（系统设置 / 用户管理…）
const groups = computed<Menu.recordRaw[]>(() => {
  const main = menuStore.allMenus[menuStore.actived]
  return (main?.children ?? []) as Menu.recordRaw[]
})

function groupTitle(group: Menu.recordRaw) {
  return resolveMenuTitle(group.meta?.title)
}

// 分组下的可见页面
function groupPages(group: Menu.recordRaw) {
  return (group.children ?? []).filter(p => p.meta?.sidebar !== false)
}

function pageTitle(page: Menu.recordRaw) {
  return resolveMenuTitle(page.meta?.title)
}

function pagePath(group: Menu.recordRaw, page: Menu.recordRaw) {
  return resolveRoutePath(group.path ?? '', page.path)
}

const activePath = computed(() => (route.meta?.activeMenu as string) || route.path)

function isGroupActive(group: Menu.recordRaw) {
  const base = group.path
  return activePath.value === base || activePath.value.startsWith(`${base}/`)
}

function isPageActive(group: Menu.recordRaw, page: Menu.recordRaw) {
  return activePath.value === pagePath(group, page)
}

function go(path: string) {
  if (path && path !== route.path) {
    router.push(path)
  }
}

// 点击分组：跳到该组第一个可见页面
function goGroup(group: Menu.recordRaw) {
  const pages = groupPages(group)
  if (pages.length) {
    go(pagePath(group, pages[0]))
  }
  else if (group.path) {
    go(group.path)
  }
}
</script>

<template>
  <transition name="header">
    <header v-if="settingsStore.mode === 'pc' && settingsStore.settings.menu.menuMode === 'head'" class="topnav">
      <router-link :to="brandTo" class="brand" :title="title">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="1.6">
          <path d="M12 2v20M2 12h20M5 5l14 14M19 5L5 19" stroke-linecap="round" />
        </svg>
        <span>{{ title }}</span>
      </router-link>

      <nav class="menu">
        <div v-for="(group, gi) in groups" :key="group.path || gi" class="grp">
          <template v-if="groupPages(group).length">
            <button type="button" class="grp-btn" :class="{ on: isGroupActive(group) }" @click="goGroup(group)">
              {{ groupTitle(group) }}
              <svg class="cv" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M6 9l6 6 6-6" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </button>
            <div class="grp-drop">
              <a
                v-for="page in groupPages(group)"
                :key="pagePath(group, page)"
                :class="{ cur: isPageActive(group, page) }"
                @click="go(pagePath(group, page))"
              >{{ pageTitle(page) }}</a>
            </div>
          </template>
          <button
            v-else
            type="button"
            class="grp-btn"
            :class="{ on: isGroupActive(group) }"
            @click="go(group.path ?? '')"
          >
            {{ groupTitle(group) }}
          </button>
        </div>
      </nav>

      <Tools class="rt" />
    </header>
  </transition>
</template>

<style lang="scss" scoped>
.topnav {
  position: relative;
  z-index: 50;
  display: flex;
  align-items: center;
  gap: 30px;
  height: var(--g-header-height);
  padding: 0 24px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  transition: background-color 0.3s, var(--el-transition-color);
}

.brand {
  display: flex;
  align-items: center;
  gap: 9px;
  font-family: var(--font-display);
  font-size: 15.5px;
  font-weight: 680;
  color: var(--fg);
  letter-spacing: -0.01em;
  text-decoration: none;
  white-space: nowrap;

  svg {
    flex: none;
  }
}

.menu {
  display: flex;
  gap: 26px;
  align-self: stretch;
}

.grp {
  position: relative;
  display: flex;
  align-items: center;

  .grp-btn {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 5px;
    height: 100%;
    padding: 0 2px;
    font: inherit;
    font-size: 14px;
    font-weight: 500;
    color: var(--text);
    background: none;
    border: 0;
    cursor: pointer;
    transition: color 0.18s var(--ease);

    .cv {
      color: var(--faint);
      transition: transform 0.18s var(--ease);
    }

    &::after {
      content: '';
      position: absolute;
      left: 0;
      right: 13px;
      bottom: 0;
      height: 2px;
      background: transparent;
      border-radius: 2px 2px 0 0;
      transition: background 0.18s var(--ease);
    }

    &.on {
      color: var(--fg);
      font-weight: 600;
    }

    &.on::after {
      background: var(--accent);
    }
  }

  &:hover .grp-btn {
    color: var(--fg);

    .cv {
      transform: rotate(180deg);
    }
  }

  .grp-drop {
    position: absolute;
    top: 100%;
    left: -8px;
    min-width: 152px;
    padding: 6px;
    display: flex;
    flex-direction: column;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    box-shadow: var(--shadow-md);
    opacity: 0;
    visibility: hidden;
    transform: translateY(4px);
    transition: opacity 0.16s var(--ease), transform 0.16s var(--ease);
    z-index: 60;

    a {
      display: block;
      padding: 8px 12px;
      font-size: 13.5px;
      font-weight: 500;
      color: var(--text);
      white-space: nowrap;
      text-decoration: none;
      cursor: pointer;
      border-radius: var(--r-sm);
      transition: background 0.12s var(--ease), color 0.12s var(--ease);

      &:hover {
        background: var(--surface-2);
        color: var(--fg);
      }

      &.cur {
        color: var(--accent-text);
        background: var(--accent-weak);
        font-weight: 550;
      }
    }
  }

  &:hover .grp-drop {
    opacity: 1;
    visibility: visible;
    transform: translateY(0);
  }
}

.rt {
  margin-left: auto;
}

@media screen and (max-width: 760px) {
  .menu {
    display: none;
  }
}

// 头部动画
.header-enter-active,
.header-leave-active {
  transition: transform 0.3s;
}

.header-enter-from,
.header-leave-to {
  transform: translateY(calc(var(--g-header-height) * -1));
}
</style>
