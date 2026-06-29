# 设计资源

EgoAdmin 的前端设计资源位于 `web/design`，GitHub Pages 发布后会作为静态预览站点挂载到 `/design/`。文档站点发布在 Pages 根路径，设计稿保留为独立入口，方便开发者在实现 Vue3 + Element Plus 页面时对照视觉稿、交互状态和响应式规则。

::: tip 访问入口
发布后可以从顶部导航的 **设计资源** 直接打开预览站点，也可以访问 `/design/`。本地仓库中对应目录是 `web/design`。
:::

## 资源结构

| 文件 | 用途 |
|------|------|
| `web/design/index.html` | 设计资源启动页 / 页面索引 |
| `web/design/design-system.html` | 设计系统、视觉 token、组件状态和交互规范 |
| `web/design/DESIGN-MANIFEST.json` | 机器可读的设计资源清单，包含页面、资源、视口和实现策略 |
| `web/design/DESIGN-HANDOFF.md` | 面向开发实现的交接说明 |
| `web/design/DESIGN.md` | 设计说明文档 |
| `web/design/critique.json` | 设计检查与评审数据 |

## 页面入口

| 页面 | 预览路径 | 说明 |
|------|----------|------|
| 设计系统 | `/design/design-system.html` | 颜色、字号、间距、圆角、阴影、组件状态 |
| 落地页 | `/design/landing.html` | 产品首页 / 外部展示页面 |
| 登录页 | `/design/login.html` | 登录表单、品牌区、错误状态参考 |
| 用户管理 | `/design/user.html` | 用户列表、筛选、表格、操作入口 |
| 角色管理 | `/design/role.html` | 角色列表、权限配置入口 |
| 组织管理 | `/design/organization.html` | 部门 / 组织结构管理 |
| 在线用户 | `/design/online_user.html` | 在线会话、强制下线等运维视图 |
| 操作日志 | `/design/operation_log.html` | 审计日志、筛选条件、详情查看 |
| 个人中心 | `/design/personal.html` | 个人资料、安全设置 |
| 状态页 | `/design/states.html` | 空状态、加载、异常、成功等通用状态 |

## 开发使用流程

实现或调整前端页面时，建议按以下顺序使用设计资源：

1. 打开 `/design/design-system.html`，提取颜色、字号、间距、圆角、阴影、动效和组件状态。
2. 打开目标页面，例如 `/design/user.html`，确认布局密度、表格列、筛选区域、按钮层级和弹窗行为。
3. 对照 `web/src` 中现有 Vue 页面和组件，不引入与当前 Element Plus 风格冲突的新视觉体系。
4. 实现后在移动端、平板、桌面宽度下检查布局，不允许出现横向滚动、按钮文字溢出或表格工具栏挤压。
5. 对用户可见页面变更补充请求 / 响应示例或截图，便于 PR 评审。

## 页面到前端模块映射

| 设计稿 | 建议实现位置 | 关注点 |
|--------|--------------|--------|
| `login.html` | `web/src/views/login` | 登录表单、错误提示、loading、密码加密流程 |
| `user.html` | `web/src/views/system/user` | 查询表单、分页表格、用户新增 / 编辑 / 禁用 |
| `role.html` | `web/src/views/system/role` | 角色 CRUD、菜单权限、按钮权限 |
| `organization.html` | `web/src/views/system/organization` | 树形组织、部门编辑、层级状态 |
| `online_user.html` | `web/src/views/monitor/online-user` | 会话列表、在线状态、强制下线 |
| `operation_log.html` | `web/src/views/monitor/operation-log` | 审计日志筛选、详情弹窗、导出入口 |
| `personal.html` | `web/src/views/account/personal` | 用户资料、安全设置、密码修改 |
| `states.html` | `web/src/components` | 空、加载、错误、成功状态组件 |

## 实现约束

::: warning 设计稿是当前状态的视觉契约
不要在实现中加入“新旧布局对比”或历史说明。文档和设计资源都只描述当前项目最新状态。
:::

前端实现时需要遵守：

| 约束 | 要求 |
|------|------|
| 组件库 | 优先使用 Vue3 + Element Plus，避免重复造基础组件 |
| 状态 | 页面需要覆盖默认、hover、focus、disabled、loading、empty、error、success |
| 布局 | 以当前管理后台的信息密度为准，避免营销式大卡片布局 |
| 响应式 | 至少检查 390px、820px、1366px、1440px 宽度 |
| 访问性 | 表单、按钮、链接保持语义化，焦点状态必须可见 |
| 数据 | 用真实接口字段和业务命名，不用无意义占位文案替代 |

## 本地预览

设计资源是纯静态 HTML，可以直接打开文件，也可以用任意静态服务预览：

```bash
# 从仓库根目录启动一个静态服务
python3 -m http.server 4174 --directory web/design
```

访问：

```text
http://localhost:4174/
http://localhost:4174/design-system.html
http://localhost:4174/user.html
```

## GitHub Pages 发布路径

当前 Pages 发布策略为：

| 内容 | 发布路径 |
|------|----------|
| VitePress 文档 | `/` |
| 中文文档 | `/guide/...` |
| 英文文档 | `/en-US/guide/...` |
| 设计资源 | `/design/...` |

发布产物由 `.github/workflows/pages.yml` 统一生成，避免多个 workflow 互相覆盖 GitHub Pages。
