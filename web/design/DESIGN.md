# EgoAdmin 设计规范 · DESIGN.md

> 开箱即用的 Go 微服务后台开发底座 —— 全项目视觉与组件规范
> 风格方向：**现代极简（Linear / Vercel）· 亮色优先 · 电光蓝 + Go 青色强调**
> 适用范围：解释 / 落地页（`home`）、Element Plus 后台全部页面、登录页
> 版本：v1.0 ｜ 状态：现行规范（落地页 / 后台页 / 登录页均按此规范实现）

---

## 0. 定位与设计目标

EgoAdmin 是开箱即用的 Go 微服务后台开发底座。整套界面 —— 落地页、Element Plus 后台、登录页 —— 遵循同一套规范：

- **主操作色** 电光蓝 `#1573E6`；**信号色** Go 青 `#19B6DD`（呼应 Go 生态）。
- **外壳** 单一中性画布 + 发丝级描边；强调色每屏出现 ≤ 2 次。
- **气质** 契约 / 代码语境构成图形语言 —— 等宽字体、代码窗、`proto / make` 片段，不用卡通插画。
- **形态** 克制圆角（卡片 12 / 控件 8）、近乎无投影（投影仅用于浮层）、简体中文优先。

**一句话目标：** 让 EgoAdmin 像「一套被认真打磨过、可以直接发布」的开发者底座产品 —— 安静、精确、信息密度高，强调色只在关键动作和代码语境出现。

---

## 1. 设计原则

1. **契约即气质（Contract as identity）** —— 产品的灵魂是 Proto First。视觉上用等宽字体、代码窗、`proto / rpc / make` 片段作为核心装饰，而不是图标插画。
2. **克制优先（Restraint over ornament）** —— 一个强调色，每屏最多出现两次（通常是「眉标 + 主按钮」或「选中态 + 主按钮」）。没有渐变背景、没有 emoji 图标、没有左侧色条卡片。
3. **发丝描边代替投影** —— 用 1px 中性描边 + 留白构建层级；投影只用于浮层（下拉、弹窗、气泡）。
4. **数字工程感** —— 所有数字、ID、时间、计量使用等宽 + `tabular-nums` 对齐；表格、日志是「功能」而非「内容」。
5. **一处决定性亮点** —— 落地页的代码窗加载动画 / 后台的命中高亮，每个场景只保留一个「值得记住」的细节。
6. **中文优先** —— 所有用户可见文案默认简体中文；品牌名 `EgoAdmin`、代码、命令、技术标识保持原样。

---

## 2. 色彩系统

### 2.1 中性色（亮色 · 画布与文本）

OKLch 为权威值，hex 为兜底（供 Element Plus SCSS 及不支持 oklch 的环境使用）。

```css
:root {
  /* 画布 / 表面 */
  --bg:          oklch(98.7% 0.002 250);  /* #FAFBFC  页面背景 */
  --surface:     oklch(100%  0      0);    /* #FFFFFF  卡片 / 内容面 */
  --surface-2:   oklch(97%   0.004 250);  /* #F4F6F8  表头 / 次级面板 / 代码窗底 */
  --surface-3:   oklch(94.5% 0.005 250);  /* #EAEDF1  分组底 / 禁用面 */

  /* 文本 */
  --fg:          oklch(22%  0.012 255);   /* #14181F  主文本 / 标题 */
  --text:        oklch(38%  0.014 255);   /* #3A4250  正文常规 */
  --muted:       oklch(54%  0.013 255);   /* #6B7280  次要 / 说明 */
  --faint:       oklch(68%  0.012 255);   /* #9AA3B2  占位符 / 禁用文字 */

  /* 描边 */
  --border:      oklch(92.5% 0.005 255);  /* #E6E8EC  发丝描边（默认） */
  --border-strong: oklch(87% 0.007 255);  /* #D5D9E0  输入框 / 强分隔 */
}
```

### 2.2 品牌强调色（电光蓝 + Go 青）

```css
:root {
  /* 主操作色 —— 电光蓝（按钮 / 链接 / 选中态） */
  --accent:        oklch(58%  0.162 245);  /* #1573E6 */
  --accent-hover:  oklch(53%  0.170 245);  /* #1267CC */
  --accent-press:  oklch(48%  0.170 245);  /* #0F58B0 */
  --accent-weak:   oklch(96%  0.028 245);  /* #E8F1FC  浅底 / 选中行底 */
  --accent-text:   oklch(50%  0.170 245);  /* #1166C4  白底上的链接文字（保证 AA） */
  --on-accent:     #ffffff;                 /* 强调色之上的文字 */

  /* 信号色 —— Go 青（数据 / 高亮 / 次级标识，呼应 Go 生态） */
  --cyan:          oklch(72%  0.130 222);  /* #19B6DD */
  --cyan-weak:     oklch(95%  0.030 222);  /* #E2F6FC */
}
```

> **强调色预算**：`--accent` 是唯一主色；`--cyan` 只在「代码窗高亮、数据点、Proto First 徽标、流水线节点」等少量语境出现，不参与按钮系统。两者绝不在同一组件里同时抢视觉。

### 2.3 状态色

```css
:root {
  --success:      oklch(66%  0.150 152);  /* #17B26A 启用 / 成功 / 健康 */
  --success-weak: oklch(95%  0.040 152);  /* #E6F7EF */
  --warning:      oklch(78%  0.150 75);   /* #F5A623 待迁移 / 提醒 */
  --warning-weak: oklch(96%  0.050 80);   /* #FDF3E2 */
  --danger:       oklch(60%  0.200 27);   /* #E5484D 错误 / 禁用 / 删除 */
  --danger-weak:  oklch(95%  0.040 27);   /* #FCEBEC */
  --info:         var(--cyan);            /* 信息提示复用 Go 青 */
}
```

### 2.4 暗色模式（亮色为主，暗色为完整副本）

提供 `.dark` 暗色切换，token 一一对应覆盖：

```css
.dark {
  --bg:          oklch(17%  0.012 255);   /* #0E1116 */
  --surface:     oklch(21%  0.013 255);   /* #161B22 */
  --surface-2:   oklch(25%  0.014 255);   /* #1C232D */
  --surface-3:   oklch(29%  0.014 255);   /* #232B36 */
  --fg:          oklch(94%  0.006 255);   /* #E6EAF0 */
  --text:        oklch(82%  0.010 255);   /* #C2C9D4 */
  --muted:       oklch(66%  0.012 255);   /* #8B95A4 */
  --faint:       oklch(52%  0.012 255);   /* #6A7382 */
  --border:      oklch(30%  0.013 255);   /* #262C36 */
  --border-strong: oklch(38% 0.014 255);  /* #38414E */
  --accent:        oklch(68% 0.150 245);  /* #4B9CF5 暗底提亮 */
  --accent-weak:   oklch(30% 0.060 245);  /* #17304E */
  --accent-text:   oklch(74% 0.130 245);  /* #6FB0F7 */
  --cyan:          oklch(76% 0.120 222);  /* #43C6E8 */
}
```

### 2.5 用色规则（do / don't）

- ✅ 页面背景永远是中性 `--bg`；卡片为纯白 `--surface`。
- ✅ 强调色用于：主按钮、链接、当前导航、选中行 / 选中项、聚焦环。
- ✅ 状态以「文字色 + 同色浅底标签」表达（见 §9.4），不要整行染色。
- ❌ 不使用蓝紫渐变、米色 / 暖橙 / 粉色背景。
- ❌ 不给每个标题配图标、不给每个卡片加左色条。
- ❌ 强调色不做大面积铺底（仅 hero 可有极淡的 `--accent-weak` / 网格底纹）。

---

## 3. 字体与排版

### 3.1 字体栈

```css
:root {
  /* 标题 / 展示（系统超家族，CJK 回退） */
  --font-display: -apple-system, BlinkMacSystemFont, "SF Pro Display",
                  "PingFang SC", "Microsoft YaHei", "Segoe UI", system-ui, sans-serif;
  /* 正文 / UI */
  --font-body:    -apple-system, BlinkMacSystemFont, "SF Pro Text",
                  "PingFang SC", "Microsoft YaHei", "Segoe UI", system-ui, sans-serif;
  /* 等宽 —— 代码、命令、ID、时间、计量（产品的「装饰字体」） */
  --font-mono:    "JetBrains Mono", "SF Mono", "Cascadia Code",
                  ui-monospace, "Roboto Mono", Menlo, Consolas, monospace;
}
```

> 现代极简刻意使用系统字体（display / text 为同超家族的不同光学尺寸，非同名族，符合规范的「display ≠ body」要求）。**等宽是 EgoAdmin 的招牌**：代码片段、`make` 命令、Snowflake ID、`referenceId`、时间戳一律 mono。

### 3.2 字阶（桌面）

| Token | 用途 | 字号 / 行高 | 字重 | 字距 |
| --- | --- | --- | --- | --- |
| `display-xl` | 落地页主标题 | `clamp(40px,4.2vw,60px)` / 1.06 | 680 | -0.022em |
| `display-l`  | 落地页区块标题 | 36px / 1.12 | 660 | -0.018em |
| `h1` | 后台页面标题 | 28px / 1.25 | 620 | -0.012em |
| `h2` | 卡片 / 区块标题 | 20px / 1.35 | 600 | -0.008em |
| `h3` | 小节 / 表单分组 | 16px / 1.45 | 600 | 0 |
| `body-l` | 落地页正文 / 引导 | 16px / 1.65 | 420 | 0 |
| `body` | 后台默认正文 / 表格 | 14px / 1.6 | 420 | 0 |
| `small` | 辅助说明 / 表头 | 13px / 1.5 | 500 | 0 |
| `eyebrow` | 眉标 / 标签 | 12px / 1.4 | 600 | **0.08em（大写 mono）** |
| `numeric` | 数字 / ID / 时间 | 继承字号 | 500 | `font-variant-numeric: tabular-nums` |

规则：标题统一 `text-wrap: balance`；正文 `text-wrap: pretty`；标题与说明的颜色对比用 `--fg` / `--muted` 拉开层级，而非全部加粗。

---

## 4. 间距与栅格

```css
:root {
  --sp-1: 4px;  --sp-2: 8px;  --sp-3: 12px; --sp-4: 16px;
  --sp-5: 20px; --sp-6: 24px; --sp-8: 32px; --sp-10: 40px;
  --sp-12: 48px; --sp-16: 64px; --sp-20: 80px; --sp-24: 96px;
}
```

- **基准 8px 节奏**（4px 用于密集控件内部）。
- **落地页**：内容容器 `max-width: 1200px`，左右留白 ≥ 24px；区块上下间距桌面 `--sp-24`（96px），段落 `--sp-6`。12 列栅格，列间距 24px。
- **后台内容区**：`max-width: 1440px`，页面内边距 `--sp-6`（24px）；卡片内边距 20–24px。
- **拆分布局**（用户 / 组织）：左侧上下文面板 `260px` + 间距 `--sp-6` + 右侧表格卡自适应。

---

## 5. 圆角 · 描边 · 阴影 · 层级

```css
:root {
  /* 圆角（克制） */
  --r-xs: 4px; --r-sm: 6px; --r-md: 8px;   /* 控件 / 按钮 / 输入框 = 8 */
  --r-lg: 12px; --r-xl: 16px;              /* 卡片 = 12，代码窗 / 大容器 = 16 */
  --r-pill: 999px;                          /* 标签 / 开关 */

  /* 描边 */
  --bd: 1px solid var(--border);
  --bd-strong: 1px solid var(--border-strong);

  /* 阴影（仅浮层 / 悬浮卡） */
  --shadow-xs: 0 1px 2px rgb(16 24 40 / 0.04);
  --shadow-sm: 0 2px 6px rgb(16 24 40 / 0.06);
  --shadow-md: 0 6px 18px -6px rgb(16 24 40 / 0.12);   /* 下拉 / 悬浮卡 */
  --shadow-lg: 0 16px 40px -12px rgb(16 24 40 / 0.18); /* 弹窗 */

  /* 聚焦环 */
  --focus: 0 0 0 3px color-mix(in oklab, var(--accent) 22%, transparent);
}
```

层级（z-index）：内容 `1` → 吸顶导航 `100` → 下拉 / 气泡 `1000` → 抽屉 `1500` → 弹窗 `2000` → 全局提示 `3000`。

---

## 6. 动效

```css
:root {
  --ease: cubic-bezier(0.2, 0, 0, 1);      /* 标准 */
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);
  --t-fast: 120ms; --t-base: 180ms; --t-slow: 260ms; --t-overlay: 320ms;
}
```

- **微交互**：按钮 / 链接 / 行 hover 用 `--t-base var(--ease)`，只过渡 `background / color / border / box-shadow / transform`。
- **行内动作显隐**：表格行操作按钮 hover 渐显，`opacity --t-base`。
- **浮层进出**：下拉 / 弹窗用 `--t-overlay var(--ease-out)`，位移 ≤ 8px。
- **决定性亮点**：落地页 hero 代码窗逐行打字 / 高亮一次性进场；其余保持安静。
- 尊重 `prefers-reduced-motion`：关闭非必要位移与逐行动画。

---

## 7. 图标与插画

- **图标**：线性、`1.5px` 描边、尺寸 16 / 18 / 20 / 24。后台用 Element Plus Icon；落地页用同风格线性图标或纯字符。颜色默认 `--muted`，激活 `--accent`。
- **禁止**：emoji 当功能图标、3D 卡通人物、手绘人物 / 场景插画、每个标题旁配图标。
- **「插画」即代码**：用代码窗、proto 片段、命令行、契约关系图（方块 + 连线，发丝描边）作为图形语言，不使用装饰性插画。
- **Logo**：几何雪花标记，统一为单色 `--fg` 或 `--accent`，配中文「核心管理平台」/ 英文 `EgoAdmin` wordmark。

---

## 8. Element Plus 接入（复制即用）

将设计 token 映射到 Element Plus CSS 变量，在全局样式的主题段中定义：

```css
:root {
  /* 主色及亮度梯度（El 据此生成组件态） */
  --el-color-primary: #1573E6;
  --el-color-primary-light-3: #5B9DED;
  --el-color-primary-light-5: #8AB9F3;
  --el-color-primary-light-7: #B9D5F8;
  --el-color-primary-light-8: #D0E3FA;
  --el-color-primary-light-9: #E8F1FC;
  --el-color-primary-dark-2:  #1267CC;

  --el-color-success: #17B26A;
  --el-color-warning: #F5A623;
  --el-color-danger:  #E5484D;
  --el-color-error:   #E5484D;
  --el-color-info:    #19B6DD;

  /* 文本 / 背景 / 描边 → 对齐设计 token */
  --el-text-color-primary:   #14181F;
  --el-text-color-regular:   #3A4250;
  --el-text-color-secondary: #6B7280;
  --el-text-color-placeholder:#9AA3B2;
  --el-bg-color:        #FFFFFF;
  --el-bg-color-page:   #FAFBFC;
  --el-bg-color-overlay:#FFFFFF;
  --el-border-color:        #E6E8EC;
  --el-border-color-light:  #EDEFF2;
  --el-border-color-lighter:#F2F4F6;
  --el-border-color-hover:  #D5D9E0;

  --el-border-radius-base:  8px;
  --el-border-radius-small: 6px;
  --el-font-size-base: 14px;
  --el-box-shadow-light: 0 2px 6px rgb(16 24 40 / 0.06);
}
```

> `--el-fill-color-light` 用浅灰填充 `#F4F6F8`（用于 hover / 选中等浅色填充，不要用主色铺底）。

---

## 9. 组件规范（Element Plus）

### 9.1 按钮 `el-button`
- 圆角 `--r-md`(8)，默认高度 36（small 28 / large 44），padding `6px 16px`。
- **主按钮**：实心 `--accent`，白字；hover `--accent-hover`，按下 `--accent-press`。每屏主按钮唯一。
- **次按钮**：白底 + `--border-strong` 描边 + `--text` 文字；hover 描边转 `--accent`、文字转 `--accent-text`。
- **文字按钮**：用于表格行内操作（编辑 / 删除 / 授权），默认 `--accent-text`，删除类 `--danger`。
- **危险操作**：实心 `--danger` 或文字 `--danger`，必经二次确认。

### 9.2 输入 / 选择 `el-input` `el-select` `el-cascader`
- 高度 36，圆角 8，描边 `--border-strong`；hover 描边加深，聚焦 `--accent` 描边 + `--focus` 环。
- 占位符 `--faint`；前后缀图标 `--muted`。
- 校验错误：描边 `--danger` + 错误文字 13px `--danger`。

### 9.3 数据表格 `el-table`（核心）
- 表头：底 `--surface-2`(#F4F6F8)，文字 13px / 600 / `--text`，高度 48。
- 行高 56，行间发丝下边框；hover 行底 `--accent-weak`（极淡蓝）。
- **行内操作** `.el-table-btn`：hover 渐显，绝对定位右侧、随行高对齐。
- 数字 / 时间 / ID 列：mono + `tabular-nums` + 右对齐（金额 / 计数）或左对齐（ID）。
- 排序 / 可筛选表头图标 `--muted`，激活 `--accent`。
- 空态 / 加载态：见 §9.10。

### 9.4 标签 / 状态 `el-tag`
统一「同色浅底 + 深色文字 + 圆角 pill」，禁止整行染色：

| 状态 | 文字 | 底 |
| --- | --- | --- |
| 启用 / 成功 / 健康 | `--success` | `--success-weak` |
| 待处理 / 提醒 | `--warning` | `--warning-weak` |
| 禁用 / 错误 | `--danger` | `--danger-weak` |
| 信息 / 标识（如「Proto First」） | `--cyan` | `--cyan-weak` |
| 中性（角色 / 组织名） | `--text` | `--surface-2` |

> 状态一律用同色浅底标签表达，让「状态」与「链接蓝」在颜色上彻底区分。

### 9.5 分页 `el-pagination`
- 右对齐：`共 N 条 · 每页 [10 ▾] · ‹ 1 2 3 ›`（中文化「Total / page」）。
- 当前页实心 `--accent`，其余 hover `--accent-weak`；尺寸 small。

### 9.6 弹窗 `el-dialog` / 抽屉 `el-drawer`
- 宽度 480（表单）/ 640（复杂）/ 抽屉 480；圆角 `--r-lg`，阴影 `--shadow-lg`，遮罩 `rgb(16 24 40 / 0.45)`。
- 头部：18px / 600 `--fg`，下方 `--bd` 分隔；关闭图标 `--muted`。
- 底部右对齐：次按钮 + 主按钮；危险确认主按钮为 `--danger`。

### 9.7 树 `el-tree`（组织 / 数据范围）
- 节点高 32；hover 底 `--surface-2`，选中文字 `--accent` + 底 `--accent-weak`。
- 左侧上下文面板：卡片化（白底、`--bd`、圆角 12），顶部带搜索框 + 「数据范围」标题。

### 9.8 导航 `el-menu`（后台外壳）
- **顶部水平导航**：白底吸顶 + 底部 `--bd`；项默认 `--text`，hover `--fg`，当前项 `--accent` 文字 + **2px `--accent` 下划线**；分组项（系统设置 / 用户管理）hover 展开子页面下拉。
- 右侧操作区：搜索 / 主题切换 / 语言 / 头像下拉，图标 `--muted`、间距 `--sp-4`。
- 如启用左侧栏模式：图标轨 70 + 子菜单 232，外壳用中性白 / `--surface-2`，选中态 `--accent-weak` 底 + `--accent` 文字。

### 9.9 卡片 / 页面框 `PageMain` `SearchBar` `Pagination`
- `PageMain`：白底卡片，`--bd`，圆角 `--r-lg`，内边距 24；头部为「页面标题(h1) + 右侧操作」。
- `SearchBar`：一行筛选项（输入 / 选择 + 主查询按钮 + 重置文字按钮），与表格卡间距 `--sp-5`。
- 代码窗（落地页招牌组件）：底 `--surface-2`，圆角 16，`--bd`；顶部 mac 三色点 + 文件名(mono `--muted`) + 右侧 `Proto First` 青色标签；内容 mono 语法高亮（关键字 `--accent`、字符串 `--success`、注释 `--faint`）。

---

### 9.10 空状态与加载状态

**空状态** —— 柔色圆底（`--surface-2`，直径 60–64）内置 1.5px 线性图标 + 标题（15 / 600 / `--fg`）+ 说明（13 / `--muted`）+（可选）操作按钮；纯 token、亮暗通用，**不使用具象插画**。三态：
- **无数据 noData**：收件箱 / 人形图标 +「暂无数据」+ 引导新增 + 主按钮（如「新增角色」）。
- **无搜索结果 noSearchResults**：放大镜 +「没有匹配结果」+「试试调整关键词或筛选条件」+ 次按钮「清空筛选」。
- **无权限 noPermission**：锁图标 +「无访问权限」+「请联系管理员」，无操作按钮。

**加载状态**：
- **骨架屏（默认）**：表格 / 整页加载用骨架屏 —— 表头保留、表体替换为等高占位行（灰条 `--surface-2` + 微光 shimmer 扫过 1.3s）；先占好结构，体感比 spinner 更快。
- **行内 spinner**：仅用于按钮内（白色顶点）、「加载更多」、局部刷新等小范围；14–18px、2px 环、`--accent` 顶点。**不使用整页居中转圈遮罩。**
- **触发时机**：首屏、切组织、应用筛选、重置等「真实请求」时刻显示骨架；纯前端的关键词即时过滤直接渲染、不显示骨架，避免闪烁。
- 尊重 `prefers-reduced-motion`：关闭 shimmer 与 spinner 动画。
- 参考实现见 `user.html`（骨架加载 + 三态空状态），可视演示见 `design-system.html` 组件区。

## 10. 布局规范

### 10.1 后台外壳
```
┌───────────────────────────────────────────────────────────┐
│  [logo 核心管理平台]   系统设置  用户管理        🔍 ◐ 中/EN  ⌄admin │  顶栏 64，白底，下边框
├───────────────────────────────────────────────────────────┤
│  page-bg #FAFBFC ·  内容 max 1440 · padding 24             │
│  ┌─────────────────────────────────────────────────────┐  │
│  │ 角色管理(h1)                         [+ 新增] [搜索🔍] │  │  PageMain 卡片
│  │ ───────────────────────────────────────────────────  │  │
│  │  表格 / 拆分布局                                       │  │
│  │ ───────────────────────────────────────────────────  │  │
│  │                          共 N 条 · 每页 ▾ · ‹ 1 2 › │  │
│  └─────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────┘
```

### 10.2 三类后台页型
1. **简单列表页**（角色 / 操作日志）：`PageMain` → 标题+操作 → 表格 → 分页。
2. **拆分列表页**（用户 / 组织）：左 260 上下文树（组织 / 数据范围）+ 右表格卡。
3. **表单 / 详情页 / 个人设置**：分组卡片（基本信息 / 安全 / 头像），label 左对齐 80–96px，项间距 `--sp-6`，主操作吸底或卡片右下。

### 10.3 登录页
- 左侧深色 / 中性「契约面板」：等宽展示一段 `role.proto` + `make run`，配极淡网格底纹与 Go 青高亮一行；右侧白底登录卡（标题、输入、主按钮、Demo 账号）。
- 体现「这是给工程师的底座」的气质。

### 10.4 落地页（解释页 `home`）区块节奏
按用户给定文案组织（中文优先）：
1. **Hero**：眉标 `READY-TO-RUN MICROSERVICE ADMIN FRAMEWORK`(mono) + 主标题「少写后台基础设施，把业务系统快速跑起来」+ 引导段 + 右侧 `role.proto` 代码窗（决定性亮点）。
2. **能力底座条**：开发契约 `Proto First` / 协议入口 `HTTP+gRPC` / 默认服务 `gateway·user·idgen` / 基础存储 `MySQL·Redis·MinIO` —— 四联等宽数据条。
3. **三大主张**：定义 proto 完成半个接口工程 / 文件能力不是一个上传接口 / 权限简单但边界完整。
4. **设计能力（6 卡）**：Proto First 契约驱动、统一上传下载与图片处理、契约化权限与数据范围、稳定高效的分布式 ID、微服务工程底座、AI 编程友好。
5. **开发流水线**：proto → make gen → 实现 → routeMenu → 验证（5 步带序号节点，Go 青连线）。
6. **快速开始**：`make install / dev-up / gen / run / e2e / migrate.new` 命令窗。
7. **继续探索**：角色管理 / 用户管理 / 组织管理 / 操作日志 —— 通向后台的入口卡。

---

## 11. 文案与本地化

- **简体中文优先**；品牌 `EgoAdmin`、代码、命令、`proto/rpc/gateway/idgen/referenceId`、版本号保持原样。
- 工程化、克制、可验证的语气；不用营销大词与未经证实的指标（不写「快 10×」「99.9%」之类 stat-slop），用真实能力描述。
- 术语统一：「契约（contract）」「数据范围（DataScope）」「断点续传（TUS）」「号段模式」「版本化迁移」。
- 按钮 / 操作动词简洁：`新增 / 编辑 / 删除 / 授权 / 导出 / 重置`（参考用户偏好，文案从简）。

---

## 12. 可访问性

- 正文对比 ≥ 4.5:1（`--text` / `--muted` 于 `--bg` 均达标）；大字 ≥ 3:1。
- 主按钮白字于 `#1573E6` ≈ 4.7:1（通过）；白底链接用 `--accent-text`(#1166C4) 保证 AA。
- 所有交互元素有可见 `:focus-visible`（`--focus` 环）；点击区 ≥ 32px（移动端 ≥ 44px）。
- 状态不只靠颜色：标签带文字；表单错误带文案；图标带 `aria-label`。
- 尊重 `prefers-reduced-motion` 与 `prefers-color-scheme`。

---

## 13. 反 AI 套路清单（交付前自检）

- ❌ 蓝紫渐变背景 / 每个区块都有渐变
- ❌ emoji 功能图标（`✨🚀🎯👋`）、3D / 手绘人物插画
- ❌ 圆角卡片 + 左侧色条；每个标题旁配图标
- ❌ 米色 / 暖橙 / 粉 / 棕的页面底色
- ❌ Inter / Roboto / Arial 作**展示**字体；正文用可
- ❌ 虚构指标、占位填充文案（「功能一 / 功能二」）
- ✅ 没有真实值时用诚实占位（`—` / 灰块 / 标注 stub），不编造

---

## 14. 交付物

- **本规范覆盖**：色彩 / 字体 / 间距 / 圆角阴影 / 动效 / 图标 / 组件（Element Plus）/ 布局 / 落地页 / 文案 / 可访问性。
- **可视化样式指南** `design-system.html`：预览本规范的 token 与核心组件、落地页 hero 与后台外壳。
- **状态预览** `states.html`：空状态 / 加载 / 出错各态在真实表格语境中的呈现。
- **页面实现**：落地页 `landing.html`、登录页 `login.html`、后台 `role / user / organization / online_user / operation_log / personal`。
- **落到代码时**：将 §8 的 Element Plus 变量定义到全局样式的主题段即可。
