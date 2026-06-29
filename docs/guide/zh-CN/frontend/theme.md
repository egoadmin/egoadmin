# 主题与设计系统

EgoAdmin 基于 CSS 自定义属性构建设计令牌系统，结合 Element Plus 主题定制，实现亮色/暗色双主题切换和统一的视觉语言。

## 概述

主题系统的核心设计理念：

- **设计令牌（Design Tokens）**：使用 CSS 自定义属性定义颜色、字体、间距、圆角、阴影等视觉属性。
- **亮暗双主题**：通过根元素 `class="dark"` 切换，CSS 变量自动重新计算。
- **品牌色**：电光蓝 `#1573E6` 为主色调，Go 青 `#19B6DD` 为信号色。
- **Element Plus 定制**：通过 SCSS `@forward` 覆盖 Element Plus 默认配色。
- **图标系统**：SvgIcon（SVG 资源）、AlIcon（iconfont）、Iconify-ep 三种图标方案。

## 核心用法

### 设计令牌体系

所有设计令牌定义在 `:root` 选择器中，位于 `styles/themes.scss`：

```scss
// styles/themes.scss
:root {
  /* 画布 / 表面 */
  --bg: #fafbfc;
  --surface: #fff;
  --surface-2: #f4f6f8;
  --surface-3: #eaedf1;

  /* 文本 */
  --fg: #14181f;
  --text: #3a4250;
  --muted: #6b7280;
  --faint: #9aa3b2;

  /* 描边 */
  --border: #e6e8ec;
  --border-strong: #d5d9e0;

  /* 强调色 - 电光蓝 */
  --accent: #1573e6;
  --accent-hover: #1267cc;
  --accent-press: #0f58b0;
  --accent-weak: #e8f1fc;
  --accent-text: #1166c4;
  --on-accent: #fff;

  /* 信号色 - Go 青 */
  --cyan: #19b6dd;
  --cyan-weak: #e2f6fc;

  /* 状态色 */
  --success: #17b26a;
  --success-weak: #e6f7ef;
  --warning: #f5a623;
  --warning-weak: #fdf3e2;
  --danger: #e5484d;
  --danger-weak: #fcebec;

  /* 字体栈 */
  --font-display: -apple-system, BlinkMacSystemFont, "SF Pro Display",
    "PingFang SC", "Microsoft YaHei", "Segoe UI", system-ui, sans-serif;
  --font-body: -apple-system, BlinkMacSystemFont, "SF Pro Text",
    "PingFang SC", "Microsoft YaHei", "Segoe UI", system-ui, sans-serif;
  --font-mono: "JetBrains Mono", "SF Mono", "Cascadia Code",
    ui-monospace, "Roboto Mono", Menlo, Consolas, monospace;

  /* 圆角 */
  --r-xs: 4px;
  --r-sm: 6px;
  --r-md: 8px;
  --r-lg: 12px;
  --r-xl: 16px;
  --r-pill: 999px;

  /* 阴影 */
  --shadow-xs: 0 1px 2px rgb(16 24 40 / 4%);
  --shadow-sm: 0 2px 6px rgb(16 24 40 / 6%);
  --shadow-md: 0 6px 18px -6px rgb(16 24 40 / 12%);
  --shadow-lg: 0 16px 40px -12px rgb(16 24 40 / 18%);

  /* 动效 */
  --ease: cubic-bezier(0.2, 0, 0, 1);
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);

  /* 间距（8px 基准） */
  --sp-1: 4px;
  --sp-2: 8px;
  --sp-3: 12px;
  --sp-4: 16px;
  --sp-5: 20px;
  --sp-6: 24px;
  --sp-8: 32px;
  --sp-10: 40px;
  --sp-12: 48px;
  --sp-16: 64px;
}
```

### 暗色主题

暗色主题通过嵌套在 `:root.dark` 下的变量覆盖实现：

```scss
// styles/themes.scss
:root {
  &.dark {
    /* 画布 / 表面 */
    --bg: #0e1116;
    --surface: #161b22;
    --surface-2: #1c232d;
    --surface-3: #232b36;

    /* 文本 */
    --fg: #e6eaf0;
    --text: #c2c9d4;
    --muted: #8b95a4;
    --faint: #6a7382;

    /* 描边 */
    --border: #262c36;
    --border-strong: #38414e;

    /* 强调色 - 暗底提亮 */
    --accent: #4b9cf5;
    --accent-hover: #62a9f7;
    --accent-press: #3c8ae6;
    --accent-weak: #17304e;
    --accent-text: #6fb0f7;

    /* 信号色 */
    --cyan: #43c6e8;
    --cyan-weak: #16323c;

    /* 状态色浅底 */
    --success-weak: #10261c;
    --warning-weak: #2a2110;
    --danger-weak: #2c1517;

    /* 阴影加强 */
    --shadow-xs: 0 1px 2px rgb(0 0 0 / 40%);
    --shadow-sm: 0 2px 6px rgb(0 0 0 / 45%);
    --shadow-md: 0 6px 18px -6px rgb(0 0 0 / 50%);
    --shadow-lg: 0 16px 40px -12px rgb(0 0 0 / 60%);
  }
}
```

主题切换由 `settingsStore` 中的 `watch` 自动处理：

```typescript
// store/modules/settings.ts
watch(
  () => settings.value.app.colorScheme,
  (val) => {
    if (val === '') {
      // 跟随系统偏好
      val = window.matchMedia('(prefers-color-scheme: dark)').matches
        ? 'dark'
        : 'light'
    }
    switch (val) {
      case 'dark':
        document.documentElement.classList.add('dark')
        break
      case 'light':
        document.documentElement.classList.remove('dark')
        break
    }
  },
  { immediate: true },
)
```

### 应用级变量

在设计令牌之上，还定义了应用级语义变量，映射到具体组件区域：

```scss
// styles/themes.scss
:root {
  /* 应用 */
  --g-app-bg: var(--surface);
  --g-main-bg: var(--bg);

  /* 顶部栏 */
  --g-header-bg: var(--surface);
  --g-header-color: var(--fg);
  --g-header-menu-color: var(--text);
  --g-header-menu-hover-color: var(--fg);
  --g-header-menu-hover-bg: var(--surface-2);
  --g-header-menu-active-color: var(--accent-text);
  --g-header-menu-active-bg: var(--surface-2);

  /* 主导航 */
  --g-main-sidebar-bg: var(--surface);
  --g-main-sidebar-menu-color: var(--text);
  --g-main-sidebar-menu-hover-color: var(--fg);
  --g-main-sidebar-menu-hover-bg: var(--surface-2);
  --g-main-sidebar-menu-active-color: var(--accent-text);
  --g-main-sidebar-menu-active-bg: var(--accent-weak);

  /* 次导航 */
  --g-sub-sidebar-bg: var(--surface);
  --g-sub-sidebar-menu-color: var(--text);
  --g-sub-sidebar-menu-active-color: var(--accent);
  --g-sub-sidebar-menu-active-bg: var(--accent-weak);

  /* 进度条 */
  --g-nprogress-color: var(--accent);
}
```

### 字体系统

字体通过 `@font-face` 加载自托管的 JetBrains Mono，全局样式定义在 `styles/globals.scss`：

```scss
// styles/globals.scss
@font-face {
  font-family: "JetBrains Mono";
  font-style: normal;
  font-weight: 400;
  font-display: swap;
  src: url("/fonts/JetBrainsMono-Regular.ttf") format("truetype");
}

@font-face {
  font-family: "JetBrains Mono";
  font-style: normal;
  font-weight: 600 700;
  font-display: swap;
  src: url("/fonts/JetBrainsMono-Bold.ttf") format("truetype");
}
```

字体栈使用策略：

| 场景 | 变量 | 用途 |
|------|------|------|
| 标题 | `--font-display` | SF Pro Display + PingFang SC |
| 正文 | `--font-body` | SF Pro Text + PingFang SC |
| 代码 | `--font-mono` | JetBrains Mono 自托管 |

### 图标系统

项目提供三种图标使用方式：

**SvgIcon** -- 用于业务图标，SVG 资源放在 `assets/icons/`：

```vue
<template>
  <SvgIcon name="user-circle" />
</template>
```

**AlIcon** -- iconfont 图标，适合大量通用图标：

```vue
<template>
  <AlIcon name="icon-settings" />
</template>
```

**Iconify-ep** -- Element Plus 图标集：

```vue
<template>
  <el-icon><i-ep-search /></el-icon>
</template>
```

### 移动端适配

通过 `data-mode` 属性触发响应式布局切换：

```typescript
// store/modules/settings.ts
watch(mode, (val) => {
  document.body.setAttribute('data-mode', val)
}, { immediate: true })

function setMode(width: number) {
  if (settings.value.layout.enableMobileAdaptation) {
    // 先检查 UA 是否为移动端
    if (/Android|webOS|iPhone|iPad|iPod/i.test(navigator.userAgent)) {
      mode.value = 'mobile'
    } else {
      // 桌面端根据宽度判断
      mode.value = width < 992 ? 'mobile' : 'pc'
    }
  }
}
```

CSS 中根据 `data-mode` 适配：

```scss
// 移动端侧边栏适配
body[data-mode="mobile"] {
  --g-sub-sidebar-width: 0px;

  .sidebar {
    position: fixed;
    z-index: 1000;
    transform: translateX(-100%);

    &.is-collapse {
      transform: translateX(0);
    }
  }
}
```

## 配置示例

### SCSS 资源自动注入

通过 Vite 配置自动注入 SCSS 变量文件，所有组件无需手动 `@use`：

```typescript
// vite.config.ts
export default defineConfig({
  css: {
    preprocessorOptions: {
      scss: {
        additionalData: `@use "@/styles/resources" as *;`,
      },
    },
  },
})
```

`styles/resources` 目录结构：

```text
web/src/styles/resources/
├── _variables.scss    # SCSS 变量（断点、z-index 等）
├── _mixins.scss       # SCSS 混入（响应式、文字截断等）
└── _functions.scss    # SCSS 函数
```

### Element Plus 主题覆盖

Element Plus 的 CSS 变量映射在 `styles/element.scss` 中，通过 `--el-*` 变量覆盖默认值：

```scss
// styles/element.scss
:root {
  /* Element Plus 主色映射 */
  --el-color-primary: var(--accent);
  --el-color-primary-light-3: color-mix(in oklab, var(--accent) 70%, white);
  --el-color-primary-light-5: color-mix(in oklab, var(--accent) 50%, white);
  --el-color-primary-light-7: color-mix(in oklab, var(--accent) 30%, white);
  --el-color-primary-light-9: var(--accent-weak);
  --el-color-primary-dark-2: var(--accent-hover);

  /* 文本颜色 */
  --el-text-color-primary: var(--fg);
  --el-text-color-regular: var(--text);
  --el-text-color-secondary: var(--muted);
  --el-text-color-placeholder: var(--faint);

  /* 边框 */
  --el-border-color: var(--border);
  --el-border-color-light: var(--border);
  --el-border-color-lighter: var(--border);

  /* 背景 */
  --el-bg-color: var(--surface);
  --el-bg-color-page: var(--bg);
  --el-bg-color-overlay: var(--surface);
  --el-fill-color-blank: var(--surface);
  --el-fill-color-light: var(--surface-2);

  /* 圆角 */
  --el-border-radius-base: var(--r-sm);
  --el-border-radius-small: var(--r-xs);
  --el-border-radius-round: var(--r-pill);

  /* 字体 */
  --el-font-family: var(--font-body);
  --el-font-size-base: 14px;
}
```

### 全局样式

`globals.scss` 定义全局基础样式，包括滚动条和布局变量：

```scss
// styles/globals.scss
@use "./themes.scss";
@use "./element-plus/index.scss";
@use "./element.scss";

:root {
  --g-header-width: 100%;
  --g-header-height: 64px;
  --g-main-sidebar-width: 70px;
  --g-sub-sidebar-width: 232px;
  --g-sidebar-logo-height: 50px;
  --g-topbar-height: 50px;
}

// 自定义滚动条
::-webkit-scrollbar {
  width: 12px;
  height: 12px;
}

::-webkit-scrollbar-thumb {
  background-color: var(--border-strong);
  background-clip: padding-box;
  border: 3px solid transparent;
  border-radius: 6px;
}

::-webkit-scrollbar-thumb:hover {
  background-color: var(--muted);
}
```

样式引入顺序：

```text
globals.scss
  +-- themes.scss              # 设计令牌（:root 变量）
  +-- element-plus/index.scss  # Element Plus SCSS 覆盖
  +-- element.scss             # Element Plus CSS 变量映射
```

## 实际场景

### 场景一：在组件中使用设计令牌

```vue
<style scoped lang="scss">
.user-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--r-md);
  padding: var(--sp-4) var(--sp-6);
  box-shadow: var(--shadow-sm);
  font-family: var(--font-body);
  color: var(--text);
  transition: box-shadow var(--ease) 0.2s;

  &:hover {
    box-shadow: var(--shadow-md);
  }

  &__name {
    color: var(--fg);
    font-size: 16px;
    font-weight: 600;
  }

  &__role {
    color: var(--muted);
    font-size: 13px;
  }

  &__status {
    display: inline-block;
    padding: var(--sp-1) var(--sp-2);
    border-radius: var(--r-sm);
    font-size: 12px;

    &--active {
      background: var(--success-weak);
      color: var(--success);
    }

    &--disabled {
      background: var(--danger-weak);
      color: var(--danger);
    }
  }
}
</style>
```

### 场景二：主题感知的 API 请求

根据主题调整请求中的偏好参数：

```typescript
// 获取系统配置时传递主题偏好
async function getSystemConfig() {
  const settingsStore = useSettingsStore()
  return await api.get('/system.v1.ConfigService/GetConfig', {
    theme: settingsStore.settings.app.colorScheme,
  })
}
```

### 场景三：手动切换主题

```vue
<template>
  <el-dropdown @command="handleThemeChange">
    <el-button :icon="currentIcon" circle />
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="light">
          <el-icon><i-ep-sunny /></el-icon>
          浅色模式
        </el-dropdown-item>
        <el-dropdown-item command="dark">
          <el-icon><i-ep-moon /></el-icon>
          深色模式
        </el-dropdown-item>
        <el-dropdown-item command="">
          <el-icon><i-ep-monitor /></el-icon>
          跟随系统
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
import useSettingsStore from '@/store/modules/settings'
import { Sunny, Moon, Monitor } from '@element-plus/icons-vue'

const settingsStore = useSettingsStore()

const currentIcon = computed(() => {
  const scheme = settingsStore.settings.app.colorScheme
  if (scheme === 'dark') return Moon
  if (scheme === 'light') return Sunny
  return Monitor
})

function handleThemeChange(theme: string) {
  settingsStore.setColorScheme(theme as Required<Settings.app>['colorScheme'])
}
</script>
```

### 场景四：代码中的等宽字体

```vue
<style scoped lang="scss">
.code-block {
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.6;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  padding: var(--sp-4);
}
</style>
```

## 工作原理

### 主题切换机制

```text
用户切换主题
  |
  +-- settingsStore.setColorScheme('dark')
  |
  +-- settings.value.app.colorScheme = 'dark'
  |
  +-- watch 触发
  |     |
  |     +-- document.documentElement.classList.add('dark')
  |
  +-- :root.dark 选择器生效
  |     |
  |     +-- 所有 --xxx 变量重新计算
  |     +-- Element Plus --el-* 变量联动更新
  |
  +-- 使用变量的所有组件样式自动刷新
```

### 样式文件加载顺序

```text
main.ts
  |
  +-- import '@/styles/globals.scss'
        |
        +-- @use "./themes.scss"
        |     +-- :root { --xxx: value }          // 亮色令牌
        |     +-- :root.dark { --xxx: value }      // 暗色令牌
        |
        +-- @use "./element-plus/index.scss"
        |     +-- Element Plus SCSS 变量覆盖
        |
        +-- @use "./element.scss"
              +-- :root { --el-xxx: var(--xxx) }   // EP CSS 变量映射
```

### 设计令牌命名规范

| 前缀 | 含义 | 示例 |
|------|------|------|
| 无前缀 | 基础设计令牌 | `--accent`, `--text`, `--border` |
| `--g-` | 应用级布局变量 | `--g-header-height`, `--g-sub-sidebar-width` |
| `--el-` | Element Plus 映射 | `--el-color-primary`, `--el-border-color` |
| `--sp-` | 间距（spacing） | `--sp-2` (8px), `--sp-4` (16px) |
| `--r-` | 圆角（radius） | `--r-sm` (6px), `--r-md` (8px) |

::: tip 间距基准
所有间距基于 8px 网格。`--sp-1` = 4px（半步），`--sp-2` = 8px（基准），`--sp-4` = 16px，以此类推。
:::

## 常见问题

### 主题切换不生效

**原因**：CSS 变量级联被覆盖，或根元素 class 未正确切换。

**排查步骤**：

1. 检查 `<html>` 元素是否有 `class="dark"`。
2. 确认 `settingsStore` 的 `watch` 已触发。
3. 检查是否有更高优先级的选择器覆盖了变量值。

```typescript
// 调试主题
console.log('colorScheme:', settingsStore.settings.app.colorScheme)
console.log('html class:', document.documentElement.className)
console.log('--accent value:', getComputedStyle(document.documentElement)
  .getPropertyValue('--accent'))
```

### Element Plus 组件颜色与设计稿不一致

**原因**：`element.scss` 中的 `--el-*` 映射缺失或 `@use` 顺序错误。

**解决方案**：

1. 确认 `globals.scss` 的引入顺序：`themes.scss` -> `element-plus/index.scss` -> `element.scss`。
2. 检查 `element.scss` 中是否遗漏了需要覆盖的 `--el-*` 变量。

### SCSS 变量在组件中不可用

**原因**：Vite 的 `additionalData` 配置缺失。

**解决方案**：

```typescript
// vite.config.ts
css: {
  preprocessorOptions: {
    scss: {
      additionalData: `@use "@/styles/resources" as *;`,
    },
  },
}
```

### 暗色模式下 Element Plus 组件出现白色闪屏

**原因**：Element Plus 默认使用亮色主题，在暗色模式下需要额外处理。

**解决方案**：确保 `element-plus/index.scss` 中正确覆盖了背景色和文本颜色相关的 `--el-*` 变量，并在 HTML 根元素上引入 `dark` class。

### 图标不显示

**排查步骤**：

1. **SvgIcon**：检查 SVG 文件是否放在 `assets/icons/`，文件名是否匹配。
2. **AlIcon**：确认 iconfont 已正确引入，CSS 文件已加载。
3. **Iconify-ep**：确认 `@iconify/vue` 和 Element Plus 图标集已安装。

## 参考链接

- [Element Plus 主题定制](https://element-plus.org/zh-CN/guide/theming.html)
- [CSS 自定义属性（MDN）](https://developer.mozilla.org/zh-CN/docs/Web/CSS/Using_CSS_custom_properties)
- [Vite CSS 预处理器配置](https://vitejs.dev/config/shared-options.html#css-preprocessoroptions)
- [前端路由与权限](./routing.md)
- [状态管理](./state-management.md)
- [前端开发总览](../frontend-development.md)
