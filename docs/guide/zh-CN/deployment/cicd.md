# CI/CD 流水线

EgoAdmin 使用 GitHub Actions 实现持续集成和持续部署，覆盖代码检查、测试、构建和镜像发布的完整流程。

## 概述

CI/CD 流水线在每次推送和 PR 时自动触发，分为三个并行/串行阶段：Lint、Test、Build。最终产出经过验证的 Docker 镜像。

| 阶段 | 触发条件 | 依赖 |
|------|----------|------|
| lint | push / PR to main | 无 |
| test | push / PR to main | 无 |
| build | lint + test 通过 | lint, test |

## 核心用法

### Make 命令

CI 流水线中的核心命令：

```bash
make install         # 安装开发工具（Buf、Wire、Atlas、protoc 插件等）
make gen             # 生成 proto、Go、Wire 代码
make lint            # 格式检查和 lint
make test            # 运行 Go 测试
make build           # 编译所有服务二进制
make image.build     # 构建 Docker 镜像
```

### 本地模拟 CI

在本地复现 CI 流水线：

```bash
# 安装工具
make install

# 生成代码（必须在 build 之前运行）
make gen

# lint 检查
make lint

# 运行测试
go test -race ./...

# 构建前端（gateway 构建需要前端产物）
cd web && pnpm install && pnpm run build

# 编译所有服务
make build

# 构建镜像
make image.build
```

::: warning 代码生成
`make gen` 必须在 `make build` 或 `make lint` 之前执行。它会生成 proto 定义、Go 代码和 Wire 依赖注入代码。忘记运行会导致编译或 lint 失败。
:::

## 配置示例

### 工作流文件

完整的工作流定义位于 `.github/workflows/ci.yml`：

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - uses: pnpm/action-setup@v4
        with:
          version: 11

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml

      - name: Build frontend
        working-directory: web
        run: pnpm install && pnpm run build

      - name: Install protoc
        run: |
          wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.20.0/protoc-3.20.0-linux-x86_64.zip -O protoc.zip
          sudo unzip protoc.zip -d /usr/local && rm protoc.zip

      - name: Install tools
        run: make install

      - name: Generate code
        run: make gen

      - name: Lint
        run: make lint

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - uses: pnpm/action-setup@v4
        with:
          version: 11

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml

      - name: Build frontend
        working-directory: web
        run: pnpm install && pnpm run build

      - name: Install protoc
        run: |
          wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.20.0/protoc-3.20.0-linux-x86_64.zip -O protoc.zip
          sudo unzip protoc.zip -d /usr/local && rm protoc.zip

      - name: Install tools
        run: make install

      - name: Generate code
        run: make gen

      - name: Test
        run: go test -race ./...

  build:
    runs-on: ubuntu-latest
    needs: [lint, test]
    strategy:
      matrix:
        service: [gateway, user, idgen]
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - uses: pnpm/action-setup@v4
        with:
          version: 11

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml

      - name: Install protoc
        run: |
          wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.20.0/protoc-3.20.0-linux-x86_64.zip -O protoc.zip
          sudo unzip protoc.zip -d /usr/local && rm protoc.zip

      - name: Build frontend
        working-directory: web
        run: pnpm install && pnpm run build

      - name: Generate code
        run: make install && make gen

      - name: Build Docker image
        run: |
          docker build \
            --build-arg SERVICE=${{ matrix.service }} \
            -f cmd/${{ matrix.service }}/Dockerfile \
            -t egoadmin-test/${{ matrix.service }}:ci .
```

### 镜像发布工作流

生产镜像发布定义在 `.github/workflows/docker.yml`，通过 Git tag 触发：

```yaml
# 触发条件（示例）
on:
  push:
    tags:
      - "v*"
```

发布的镜像标签格式：

| Tag | 说明 |
|-----|------|
| `latest` | 最新 main 分支构建 |
| `v1.2.3` | 语义化版本 tag |
| `sha-abc1234` | commit SHA 短标识 |

## 实际应用

### PR 提交流程

```text
开发者推送 PR
  ├── GitHub Actions 触发 CI
  ├── lint job
  │   ├── checkout → setup-go → setup-node/pnpm
  │   ├── pnpm install && pnpm run build     (前端)
  │   ├── make install → make gen             (代码生成)
  │   └── make lint                           (格式检查)
  ├── test job（与 lint 并行）
  │   ├── checkout → setup-go → setup-node/pnpm
  │   ├── make install → make gen
  │   └── go test -race ./...                 (并发安全测试)
  └── build job（lint + test 通过后）
      ├── matrix: [gateway, user, idgen]      (三服务并行构建)
      └── docker build -f cmd/$service/Dockerfile
```

### 分支保护

main 分支配置了分支保护规则：

- 所有 PR 必须通过 CI（lint + test + build）。
- 至少一个 reviewer 批准后才能合并。
- 合并后自动触发 main 分支的 CI 和镜像构建。

### 发布流程

```text
维护者创建 Git tag
  ├── git tag v1.2.3 && git push origin v1.2.3
  ├── docker.yml workflow 触发
  ├── matrix 构建三个服务镜像
  ├── 推送到 ghcr.io/egoadmin/
  └── 镜像标签：v1.2.3, latest, sha-xxxxxxx
```

## 工作原理

### 流水线架构

```text
         ┌─────────┐     ┌─────────┐
         │  lint    │     │  test   │
         │ (并行)   │     │ (并行)   │
         └────┬────┘     └────┬────┘
              │               │
              └───────┬───────┘
                      │
              ┌───────┴───────┐
              │    build      │
              │  matrix:      │
              │  gateway      │
              │  user         │
              │  idgen        │
              └───────────────┘
```

lint 和 test 并行执行，build 在两者都通过后执行，且使用 matrix 策略并行构建三个服务。

### 工具链安装

每个 job 都需要安装以下工具链：

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | 1.26 | 编译、测试 |
| Node.js | 22 | 前端构建 |
| pnpm | 11 | 前端包管理 |
| protoc | 3.20.0 | protobuf 编译 |
| Buf | 最新 | protobuf lint 和生成 |
| Wire | 最新 | 依赖注入代码生成 |
| Atlas | 最新 | 数据库迁移 |

### 前端构建依赖

Gateway 的构建需要前端产物（`web/dist/index.html`），因此所有 job 都包含前端构建步骤：

```bash
cd web && pnpm install && pnpm run build
```

## 常见问题

### 代码生成失败

如果 CI 中 `make gen` 失败，检查以下内容：

```bash
# 本地确认生成的文件是否最新
make gen
git diff --stat    # 应该没有变更
```

::: tip
生成的代码（`wire_gen.go`、proto 输出）应该提交到仓库中。如果 CI 和本地生成结果不一致，通常是工具版本不匹配。
:::

### 测试超时

CI 中使用 `go test -race ./...` 运行所有测试。如果测试超时：

```bash
# 本地使用更长超时
go test -race -timeout 30m ./...

# 或只运行特定包
go test -race ./internal/app/gateway/...
```

### 前端构建失败

确保 `web/pnpm-lock.yaml` 已提交：

```bash
cd web
pnpm install
git add pnpm-lock.yaml
```

### 镜像构建失败

Docker 构建依赖前端产物和代码生成：

```bash
# 完整的本地镜像构建流程
cd web && pnpm install && pnpm run build
cd ..
make install
make gen
make image.build
```

### 缓存优化

CI 中使用了以下缓存策略：

| 缓存内容 | 机制 | 说明 |
|----------|------|------|
| Go modules | `actions/setup-go` 内置 | 读取 `go.sum` |
| pnpm store | `actions/setup-node` cache | 指定 `web/pnpm-lock.yaml` |
| Docker layers | BuildKit 缓存 | 无显式缓存配置 |

::: info 提升构建速度
如需进一步优化，可以在 build job 中添加 Docker layer 缓存或 `actions/cache` 缓存 Go build cache。
:::

## 参考链接

- [GitHub Actions 文档](https://docs.github.com/en/actions)
- [EgoAdmin CI 工作流](https://github.com/egoadmin/egoadmin/blob/main/.github/workflows/ci.yml)
- [测试与部署](/guide/zh-CN/testing-deployment)
- [Docker 容器化](/guide/zh-CN/deployment/docker)
