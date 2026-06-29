# Theme and Design System

EgoAdmin builds its design token system on CSS custom properties, combined with Element Plus theme customization, to implement light/dark dual-theme switching and a unified visual language.

## Overview

Core design philosophy of the theme system:

- **Design Tokens**: CSS custom properties define colors, typography, spacing, border-radius, and shadows.
- **Light/Dark Dual Theme**: Toggled via root element `class="dark"`, CSS variables recalculate automatically.
- **Brand Colors**: Electric blue `#1573E6` as the primary accent, Go cyan `#19B6DD` as the signal color.
- **Element Plus Customization**: SCSS `@forward` overrides for the Element Plus default palette.
- **Icon System**: Three approaches -- SvgIcon (SVG assets), AlIcon (iconfont), and Iconify-ep.

## Core Usage

### Design Token System

All design tokens are defined in the `:root` selector within `styles/themes.scss`:

```scss
// styles/themes.scss
:root {
  /* Canvas / Surfaces */
  --bg: #fafbfc;
  --surface: #fff;
  --surface-2: #f4f6f8;
  --surface-3: #eaedf1;

  /* Text */
  --fg: #14181f;
  --text: #3a4250;
  --muted: #6b7280;
  --faint: #9aa3b2;

  /* Borders */
  --border: #e6e8ec;
  --border-strong: #d5d9e0;

  /* Accent - Electric Blue */
  --accent: #1573e6;
  --accent-hover: #1267cc;
  --accent-press: #0f58b0;
  --accent-weak: #e8f1fc;
  --accent-text: #1166c4;
  --on-accent: #fff;

  /* Signal - Go Cyan */
  --cyan: #19b6dd;
  --cyan-weak: #e2f6fc;

  /* Status Colors */
  --success: #17b26a;
  --success-weak: #e6f7ef;
  --warning: #f5a623;
  --warning-weak: #fdf3e2;
  --danger: #e5484d;
  --danger-weak: #fcebec;

  /* Font Stacks */
  --font-display: -apple-system, BlinkMacSystemFont, "SF Pro Display",
    "PingFang SC", "Microsoft YaHei", "Segoe UI", system-ui, sans-serif;
  --font-body: -apple-system, BlinkMacSystemFont, "SF Pro Text",
    "PingFang SC", "Microsoft YaHei", "Segoe UI", system-ui, sans-serif;
  --font-mono: "JetBrains Mono", "SF Mono", "Cascadia Code",
    ui-monospace, "Roboto Mono", Menlo, Consolas, monospace;

  /* Border Radius */
  --r-xs: 4px;
  --r-sm: 6px;
  --r-md: 8px;
  --r-lg: 12px;
  --r-xl: 16px;
  --r-pill: 999px;

  /* Shadows */
  --shadow-xs: 0 1px 2px rgb(16 24 40 / 4%);
  --shadow-sm: 0 2px 6px rgb(16 24 40 / 6%);
  --shadow-md: 0 6px 18px -6px rgb(16 24 40 / 12%);
  --shadow-lg: 0 16px 40px -12px rgb(16 24 40 / 18%);

  /* Motion */
  --ease: cubic-bezier(0.2, 0, 0, 1);
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);

  /* Spacing (8px grid) */
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

### Dark Theme

The dark theme works by nesting overrides inside `:root.dark`:

```scss
// styles/themes.scss
:root {
  &.dark {
    /* Canvas / Surfaces */
    --bg: #0e1116;
    --surface: #161b22;
    --surface-2: #1c232d;
    --surface-3: #232b36;

    /* Text */
    --fg: #e6eaf0;
    --text: #c2c9d4;
    --muted: #8b95a4;
    --faint: #6a7382;

    /* Borders */
    --border: #262c36;
    --border-strong: #38414e;

    /* Accent - brightened for dark backgrounds */
    --accent: #4b9cf5;
    --accent-hover: #62a9f7;
    --accent-press: #3c8ae6;
    --accent-weak: #17304e;
    --accent-text: #6fb0f7;

    /* Signal */
    --cyan: #43c6e8;
    --cyan-weak: #16323c;

    /* Status - lighter backgrounds */
    --success-weak: #10261c;
    --warning-weak: #2a2110;
    --danger-weak: #2c1517;

    /* Stronger shadows */
    --shadow-xs: 0 1px 2px rgb(0 0 0 / 40%);
    --shadow-sm: 0 2px 6px rgb(0 0 0 / 45%);
    --shadow-md: 0 6px 18px -6px rgb(0 0 0 / 50%);
    --shadow-lg: 0 16px 40px -12px rgb(0 0 0 / 60%);
  }
}
```

Theme switching is handled automatically by a `watch` in the settings store:

```typescript
// store/modules/settings.ts
watch(
  () => settings.value.app.colorScheme,
  (val) => {
    if (val === '') {
      // Follow system preference
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

### Application-Level Variables

On top of the design tokens, application-level semantic variables map to specific component regions:

```scss
// styles/themes.scss
:root {
  /* Application */
  --g-app-bg: var(--surface);
  --g-main-bg: var(--bg);

  /* Header Bar */
  --g-header-bg: var(--surface);
  --g-header-color: var(--fg);
  --g-header-menu-color: var(--text);
  --g-header-menu-hover-color: var(--fg);
  --g-header-menu-hover-bg: var(--surface-2);
  --g-header-menu-active-color: var(--accent-text);
  --g-header-menu-active-bg: var(--surface-2);

  /* Main Navigation */
  --g-main-sidebar-bg: var(--surface);
  --g-main-sidebar-menu-color: var(--text);
  --g-main-sidebar-menu-hover-color: var(--fg);
  --g-main-sidebar-menu-hover-bg: var(--surface-2);
  --g-main-sidebar-menu-active-color: var(--accent-text);
  --g-main-sidebar-menu-active-bg: var(--accent-weak);

  /* Sub Navigation */
  --g-sub-sidebar-bg: var(--surface);
  --g-sub-sidebar-menu-color: var(--text);
  --g-sub-sidebar-menu-active-color: var(--accent);
  --g-sub-sidebar-menu-active-bg: var(--accent-weak);

  /* Progress Bar */
  --g-nprogress-color: var(--accent);
}
```

### Typography System

Fonts are loaded via `@font-face` with self-hosted JetBrains Mono. Global styles are defined in `styles/globals.scss`:

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

Font stack usage:

| Context | Variable | Purpose |
|---------|----------|---------|
| Headings | `--font-display` | SF Pro Display + PingFang SC |
| Body text | `--font-body` | SF Pro Text + PingFang SC |
| Code | `--font-mono` | JetBrains Mono self-hosted |

### Icon System

The project provides three icon usage approaches:

**SvgIcon** -- for business icons, SVG assets placed in `assets/icons/`:

```vue
<template>
  <SvgIcon name="user-circle" />
</template>
```

**AlIcon** -- iconfont icons, suitable for large sets of general icons:

```vue
<template>
  <AlIcon name="icon-settings" />
</template>
```

**Iconify-ep** -- Element Plus icon set:

```vue
<template>
  <el-icon><i-ep-search /></el-icon>
</template>
```

### Mobile Adaptation

Responsive layout switching is triggered by the `data-mode` attribute:

```typescript
// store/modules/settings.ts
watch(mode, (val) => {
  document.body.setAttribute('data-mode', val)
}, { immediate: true })

function setMode(width: number) {
  if (settings.value.layout.enableMobileAdaptation) {
    // Check UA for mobile devices first
    if (/Android|webOS|iPhone|iPad|iPod/i.test(navigator.userAgent)) {
      mode.value = 'mobile'
    } else {
      // Desktop: decide by viewport width
      mode.value = width < 992 ? 'mobile' : 'pc'
    }
  }
}
```

CSS adaptation using `data-mode`:

```scss
// Mobile sidebar adaptation
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

## Configuration Examples

### SCSS Resources Auto-Injection

SCSS variable files are auto-injected via Vite configuration so all components can use them without explicit `@use`:

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

`styles/resources` directory structure:

```text
web/src/styles/resources/
├── _variables.scss    # SCSS variables (breakpoints, z-index, etc.)
├── _mixins.scss       # SCSS mixins (responsive, text truncation, etc.)
└── _functions.scss    # SCSS functions
```

### Element Plus Theme Override

Element Plus CSS variable mappings are in `styles/element.scss`, overriding defaults via `--el-*` variables:

```scss
// styles/element.scss
:root {
  /* Element Plus primary color mapping */
  --el-color-primary: var(--accent);
  --el-color-primary-light-3: color-mix(in oklab, var(--accent) 70%, white);
  --el-color-primary-light-5: color-mix(in oklab, var(--accent) 50%, white);
  --el-color-primary-light-7: color-mix(in oklab, var(--accent) 30%, white);
  --el-color-primary-light-9: var(--accent-weak);
  --el-color-primary-dark-2: var(--accent-hover);

  /* Text colors */
  --el-text-color-primary: var(--fg);
  --el-text-color-regular: var(--text);
  --el-text-color-secondary: var(--muted);
  --el-text-color-placeholder: var(--faint);

  /* Borders */
  --el-border-color: var(--border);
  --el-border-color-light: var(--border);
  --el-border-color-lighter: var(--border);

  /* Backgrounds */
  --el-bg-color: var(--surface);
  --el-bg-color-page: var(--bg);
  --el-bg-color-overlay: var(--surface);
  --el-fill-color-blank: var(--surface);
  --el-fill-color-light: var(--surface-2);

  /* Border radius */
  --el-border-radius-base: var(--r-sm);
  --el-border-radius-small: var(--r-xs);
  --el-border-radius-round: var(--r-pill);

  /* Typography */
  --el-font-family: var(--font-body);
  --el-font-size-base: 14px;
}
```

### Global Styles

`globals.scss` defines global base styles, including scrollbars and layout variables:

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

// Custom scrollbar
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

Style import order:

```text
globals.scss
  +-- themes.scss              # Design tokens (:root variables)
  +-- element-plus/index.scss  # Element Plus SCSS overrides
  +-- element.scss             # Element Plus CSS variable mapping
```

## Real-World Examples

### Example 1: Using Design Tokens in Components

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

### Example 2: Theme-Aware API Requests

Pass theme preference when fetching system configuration:

```typescript
async function getSystemConfig() {
  const settingsStore = useSettingsStore()
  return await api.get('/system.v1.ConfigService/GetConfig', {
    theme: settingsStore.settings.app.colorScheme,
  })
}
```

### Example 3: Manual Theme Toggle

```vue
<template>
  <el-dropdown @command="handleThemeChange">
    <el-button :icon="currentIcon" circle />
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="light">
          <el-icon><i-ep-sunny /></el-icon>
          Light Mode
        </el-dropdown-item>
        <el-dropdown-item command="dark">
          <el-icon><i-ep-moon /></el-icon>
          Dark Mode
        </el-dropdown-item>
        <el-dropdown-item command="">
          <el-icon><i-ep-monitor /></el-icon>
          System Default
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

### Example 4: Monospace Font in Code Blocks

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

## How It Works

### Theme Switching Mechanism

```text
User toggles theme
  |
  +-- settingsStore.setColorScheme('dark')
  |
  +-- settings.value.app.colorScheme = 'dark'
  |
  +-- watch triggers
  |     |
  |     +-- document.documentElement.classList.add('dark')
  |
  +-- :root.dark selector activates
  |     |
  |     +-- All --xxx variables recalculate
  |     +-- Element Plus --el-* variables update in tandem
  |
  +-- All components using variables refresh styles automatically
```

### Style File Loading Order

```text
main.ts
  |
  +-- import '@/styles/globals.scss'
        |
        +-- @use "./themes.scss"
        |     +-- :root { --xxx: value }          # Light tokens
        |     +-- :root.dark { --xxx: value }      # Dark tokens
        |
        +-- @use "./element-plus/index.scss"
        |     +-- Element Plus SCSS variable overrides
        |
        +-- @use "./element.scss"
              +-- :root { --el-xxx: var(--xxx) }   # EP CSS variable mapping
```

### Design Token Naming Convention

| Prefix | Meaning | Examples |
|--------|---------|----------|
| None | Base design tokens | `--accent`, `--text`, `--border` |
| `--g-` | Application layout variables | `--g-header-height`, `--g-sub-sidebar-width` |
| `--el-` | Element Plus mappings | `--el-color-primary`, `--el-border-color` |
| `--sp-` | Spacing | `--sp-2` (8px), `--sp-4` (16px) |
| `--r-` | Border radius | `--r-sm` (6px), `--r-md` (8px) |

::: tip Spacing Grid
All spacing is based on an 8px grid. `--sp-1` = 4px (half step), `--sp-2` = 8px (base), `--sp-4` = 16px, and so on.
:::

## Common Issues

### Theme Toggle Not Working

**Cause**: CSS variable cascade is overridden, or the root element class is not toggling correctly.

**Troubleshooting**:

1. Check if `<html>` element has `class="dark"`.
2. Confirm the `settingsStore` `watch` has fired.
3. Check for higher-specificity selectors overriding variable values.

```typescript
// Debug theme
console.log('colorScheme:', settingsStore.settings.app.colorScheme)
console.log('html class:', document.documentElement.className)
console.log('--accent value:', getComputedStyle(document.documentElement)
  .getPropertyValue('--accent'))
```

### Element Plus Component Colors Mismatch

**Cause**: `--el-*` mappings in `element.scss` are missing, or the `@use` order is incorrect.

**Solution**:

1. Confirm `globals.scss` import order: `themes.scss` -> `element-plus/index.scss` -> `element.scss`.
2. Check that `element.scss` covers all needed `--el-*` variable overrides.

### SCSS Variables Not Available in Components

**Cause**: Vite's `additionalData` configuration is missing.

**Solution**:

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

### White Flash on Dark Mode with Element Plus Components

**Cause**: Element Plus defaults to light theme; dark mode requires additional handling.

**Solution**: Ensure `element-plus/index.scss` correctly overrides background and text color related `--el-*` variables, and that the `dark` class is applied to the HTML root element.

### Icons Not Displaying

**Troubleshooting**:

1. **SvgIcon**: Check that SVG files are in `assets/icons/` and filenames match.
2. **AlIcon**: Confirm iconfont is properly imported and CSS file is loaded.
3. **Iconify-ep**: Confirm `@iconify/vue` and the Element Plus icon set are installed.

## Reference Links

- [Element Plus Theming Guide](https://element-plus.org/guide/theming.html)
- [CSS Custom Properties (MDN)](https://developer.mozilla.org/en-US/docs/Web/CSS/Using_CSS_custom_properties)
- [Vite CSS Preprocessor Options](https://vitejs.dev/config/shared-options.html#css-preprocessoroptions)
- [Frontend Routing and Permissions](./routing.md)
- [State Management](./state-management.md)
- [Frontend Development Overview](../frontend-development.md)
