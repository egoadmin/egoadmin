---
layout: home

title: EgoAdmin
titleTemplate: 基于 EGO 的 Go 微服务后台管理模板

hero:
  name: EgoAdmin
  text: 企业级 Go 微服务后台管理模板
  tagline: gateway + user + idgen · 内嵌 Vue3 + Element Plus 前端 · Proto 优先 · Atlas 迁移 · DTM 分布式事务 · Docker Compose 一键部署
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/getting-started
    - theme: alt
      text: 设计资源
      link: /design/
    - theme: alt
      text: GitHub
      link: https://github.com/egoadmin/egoadmin

features:
  - icon: 🤖
    title: Agent First
    details: 内置结构化 AI skill 文档，覆盖 proto 契约、权限、分布式事务、数据权限、迁移、停机等关键链路，让 AI 按项目约定读对参考、改对位置。
  - icon: 📋
    title: Proto 优先的 API 契约
    details: 所有业务接口始于 .proto 文件，gRPC + grpc-gateway 双协议，HTTP 作为 gRPC compatibility，POST body 请求。
  - icon: 🎨
    title: 内嵌 Vue3 管理后台
    details: 同仓 web/ 目录，Vue3 + Element Plus + Pinia + Vue Router，构建产物 dist 通过 go:embed 打包进 gateway 单进程分发。
  - icon: 🔐
    title: 闭环权限体系
    details: authsession Bearer 认证 + Casbin API 权限 + DataScope 数据权限 + routeMenu 前端菜单/按钮 + permission-contract 角色校验，后端和前端权限闭环。
  - icon: 🗄️
    title: Atlas 版本化迁移
    details: gorm + atlas-provider-gorm 生成版本化 SQL 迁移文件，每服务独立数据库边界，支持 MySQL / PostgreSQL / SQLite / SQL Server 多方言。
  - icon: 🔄
    title: DTM 分布式事务
    details: 集成 DTM 分布式事务管理器，支持 Saga / TCC / 事务消息模式，等幂、空补偿、悬挂防护屏障，application 层编排事务。
  - icon: ⚙️
    title: 组件化基础设施
    details: AuthSession / LoginCrypto / IDGen / JetCache / AsyncQ / ETUSUpload / MeiliSearch / CDN 等内置业务组件，即插即用。
  - icon: 📦
    title: Docker Compose 一键部署
    details: make deploy-up 一键拉起全部服务和中间件，ghcr.io 多架构镜像，支持 linux/amd64 和 linux/arm64。
---

## 服务职责速查

| 服务 | HTTP 端口 | gRPC 端口 | 核心职责 | 数据库 |
|------|-----------|-----------|---------|--------|
| `gateway` | 9001 | 9002 | 外部 HTTP 兼容入口，内嵌前端静态服务，API 路由与鉴权聚合，TUS 文件上传，CDN 签名 | `egoadmin_gateway` |
| `user` | 9101 | 9102 | 用户 CRUD、角色/部门管理、JWT 认证与会话、Casbin RBAC、审计日志、登录加密、数据权限 | `egoadmin_user` |
| `idgen` | 9201 | 9202 | 雪花算法 ID 生成、号段模式、机器租约管理、命名空间隔离 | `egoadmin_idgen` |

## 技术栈一览

| 层次 | 技术 |
|------|------|
| 语言 | Go 1.26 |
| 框架 | EGO (github.com/gotomicro/ego) |
| HTTP 引擎 | Gin + grpc-gateway |
| 服务通信 | gRPC（内部）、HTTP POST（外部兼容） |
| 服务发现 | etcd |
| 持久化 | MySQL + GORM |
| 迁移 | Atlas + atlas-provider-gorm |
| 缓存 | Redis + JetCache（本地 + 远程双级） |
| 对象存储 | MinIO |
| 搜索 | Meilisearch |
| 异步队列 | Asynq |
| 链路追踪 | OpenTelemetry + Jaeger |
| 分布式事务 | DTM（Saga / TCC / 事务消息） |
| 前端 | Vue 3 + TypeScript + Element Plus + Pinia + Vue Router |
| 前端构建 | Vite + pnpm |
| 工具链 | Buf（proto 生成）、Wire（依赖注入）、golangci-lint |
| 容器 | Docker + Compose + 多架构镜像（amd64 / arm64） |

## 推荐阅读路径

如果你是首次接触此仓库，推荐依序阅读：

1. **[快速开始](/guide/getting-started)** — 环境准备、一键部署、本地开发、最小验证。
2. **[核心功能](/guide/features)** — 全面了解项目能力。
3. **[微服务架构](/guide/architecture)** — 理解服务边界、通信方式和目录结构。
4. **[API 开发工作流](/guide/api-development)** — 从 proto 到前端的完整链路。
5. **[设计资源](/guide/design-resources)** — 查看管理后台设计稿、设计系统和页面资源映射。
6. 按需查阅：权限、数据库、前端、DTM、组件、配置、测试部署等深度章节。

## 快速入口

```bash
# 一键部署
git clone https://github.com/egoadmin/egoadmin.git
cd egoadmin
export DOCKER_REGISTRY=ghcr.io/egoadmin
make deploy-up

# 访问 http://localhost:9001，默认 admin / 123456
```

```bash
# 本地开发
make install
make dev-up
make run
```
