# 测试与部署

## 测试分层

| 层级 | 命令 | 用途 |
|------|------|------|
| 单元测试 | `go test ./...` | 领域、用例、工具函数 |
| 竞态检测 | `go test -race ./...` | 并发安全 |
| 服务结构检查 | `make service.check SERVICE=user` | 目录、ProviderSet、注册、健康检查 |
| 前端类型检查 | `cd web && pnpm run type-check` | TypeScript |
| 前端构建 | `cd web && pnpm run build` | 打包 + 权限合约 |
| e2e | `make e2e E2E_TIMEOUT=20m` | gateway 用户可见链路 |

## 单元测试模式

EgoAdmin 使用 `testify` 作为测试框架。测试文件位于各业务包内，与源码同目录。

**基本结构**：

```go
package application

import (
  "testing"

  "github.com/stretchr/testify/assert"
  "github.com/stretchr/testify/require"
)

func TestUserUseCase_CreateUser(t *testing.T) {
  // assert 用于非致命断言
  assert.Equal(t, expected, actual)

  // require 用于致命断言，失败立即停止
  require.NoError(t, err)
}
```

::: tip
`require` 失败时立即调用 `t.FailNow()`，`assert` 失败时继续执行。对于前置条件（如错误检查），使用 `require`；对于结果验证，使用 `assert`。
:::

**使用 Mock**：

```go
package application

import (
  "testing"

  "github.com/stretchr/testify/mock"
  "github.com/stretchr/testify/require"
)

type MockUserRepo struct {
  mock.Mock
}

func (m *MockUserRepo) FindByID(ctx context.Context, id uint64) (*domain.User, error) {
  args := m.Called(ctx, id)
  if args.Get(0) == nil {
    return nil, args.Error(1)
  }
  return args.Get(0).(*domain.User), args.Error(1)
}

func TestUserUseCase_GetUser(t *testing.T) {
  mockRepo := new(MockUserRepo)
  mockRepo.On("FindByID", mock.Anything, uint64(1)).
    Return(&domain.User{ID: 1, Name: "test"}, nil)

  uc := NewUserUseCase(mockRepo)
  user, err := uc.GetUser(context.Background(), 1)

  require.NoError(t, err)
  assert.Equal(t, "test", user.Name)
  mockRepo.AssertExpectations(t)
}
```

## 表驱动测试

表驱动测试是 Go 的惯用模式，EgoAdmin 推荐在所有测试中使用：

```go
func TestCalculateDiscount(t *testing.T) {
  tests := []struct {
    name     string
    amount   float64
    level    int
    want     float64
    wantErr  bool
  }{
    {
      name:   "normal user gets no discount",
      amount: 100.0,
      level:  0,
      want:   100.0,
    },
    {
      name:   "VIP user gets 10% discount",
      amount: 100.0,
      level:  1,
      want:   90.0,
    },
    {
      name:    "negative amount is invalid",
      amount:  -10.0,
      level:   0,
      wantErr: true,
    },
    {
      name:    "zero amount is valid",
      amount:  0,
      level:   0,
      want:    0,
    },
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      got, err := CalculateDiscount(tt.amount, tt.level)
      if tt.wantErr {
        require.Error(t, err)
        return
      }
      require.NoError(t, err)
      assert.InDelta(t, tt.want, got, 0.01)
    })
  }
}
```

**测试数据库层**：

```go
func TestUserRepository_Create(t *testing.T) {
  db := setupTestDB(t) // 使用 testcontainers 或内存 SQLite
  repo := NewUserRepository(db)

  tests := []struct {
    name    string
    user    *domain.User
    wantErr bool
  }{
    {
      name:    "create valid user",
      user:    &domain.User{Name: "alice", Email: "alice@example.com"},
      wantErr: false,
    },
    {
      name:    "duplicate email fails",
      user:    &domain.User{Name: "bob", Email: "alice@example.com"},
      wantErr: true,
    },
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      err := repo.Create(context.Background(), tt.user)
      if tt.wantErr {
        require.Error(t, err)
        return
      }
      require.NoError(t, err)
      assert.NotZero(t, tt.user.ID)
    })
  }
}
```

## 集成测试

集成测试需要真实的中间件（MySQL、Redis）。推荐使用 testcontainers 或已启动的 dev 环境。

**使用 dev 环境**：

```bash
# 先启动开发中间件
make dev-up

# 运行集成测试
go test -tags=integration ./internal/app/user/...

# 清理
make dev-down
```

**使用 testcontainers**：

```go
//go:build integration

package adapter

import (
  "testing"

  "github.com/stretchr/testify/require"
  "github.com/testcontainers/testcontainers-go"
)

func setupMySQLContainer(t *testing.T) *sql.DB {
  t.Helper()
  // 启动 MySQL test container
  // 返回 *sql.DB 连接
  // 测试结束自动清理容器
  // ...
}

func TestUserMySQLRepository_Integration(t *testing.T) {
  if testing.Short() {
    t.Skip("skipping integration test in short mode")
  }

  db := setupMySQLContainer(t)
  defer db.Close()

  repo := NewUserMySQLRepository(db)
  // 执行测试...
}
```

## 服务结构检查

```bash
make service.check SERVICE=gateway
make service.check SERVICE=user
make service.check SERVICE=idgen
```

检查内容包括：

- `cmd/<service>/Dockerfile`
- `internal/app/<service>/server`
- `controller`
- `application`
- `domain`
- `adapter`
- `schema`
- `atlas/migrations/<service>/atlas.sum`
- gRPC service registration
- HTTP `/healthz` 和 `/readyz`
- ProviderSet
- 架构依赖约束

## e2e 拓扑

```text
test client
  -> gateway HTTP compatibility
  -> user gRPC via etcd
  -> service DB/Redis
```

e2e 测试位于：

```text
test/e2e/gateway/
```

适合覆盖：

- 登录/登出。
- 用户管理。
- 角色权限。
- 部门树。
- 上传/CDN。
- 权限拒绝。
- 数据权限。

**E2E 测试结构**：

```text
test/e2e/gateway/
├── main_test.go          # 测试入口和 setup
├── auth_test.go          # 登录/登出/刷新 token
├── user_test.go          # 用户 CRUD
├── role_test.go          # 角色权限
├── department_test.go    # 部门树
├── upload_test.go        # 上传/CDN
└── permission_test.go    # 权限拒绝/数据权限
```

**运行 E2E 测试**：

```bash
# 运行所有 e2e 测试（需要先启动服务）
make e2e E2E_TIMEOUT=20m

# 指定超时时间
make e2e E2E_TIMEOUT=30m
```

::: warning
E2E 测试需要 gateway、user、idgen 三个服务以及所有中间件（MySQL、Redis、etcd）已启动。使用 `make deploy-up` 或 `make dev-up` + `make run` 启动。
:::

**E2E 测试编写示例**：

```go
package gateway

import (
  "net/http"
  "testing"

  "github.com/stretchr/testify/assert"
  "github.com/stretchr/testify/require"
)

func TestLoginLogout(t *testing.T) {
  baseURL := getBaseURL()

  // 登录
  resp, err := http.PostForm(baseURL+"/api/auth/login", map[string][]string{
    "username": {"admin"},
    "password": {"123456"},
  })
  require.NoError(t, err)
  defer resp.Body.Close()
  assert.Equal(t, http.StatusOK, resp.StatusCode)

  // 解析 token
  var loginResp LoginResponse
  decodeJSON(resp.Body, &loginResp)
  require.NotEmpty(t, loginResp.AccessToken)

  // 登出
  req, _ := http.NewRequest("POST", baseURL+"/api/auth/logout", nil)
  req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
  logoutResp, err := http.DefaultClient.Do(req)
  require.NoError(t, err)
  defer logoutResp.Body.Close()
  assert.Equal(t, http.StatusOK, logoutResp.StatusCode)
}
```

## 构建

```bash
make build
make build SERVICE=user
make build.alpine SERVICE=gateway
make build.alpine-arm64 SERVICE=idgen
```

gateway 构建前会确保 `web/dist/index.html` 存在。

## 镜像

```bash
make image.build
make image.build SERVICE=user
```

发布镜像：

| 镜像 | 说明 |
|------|------|
| `ghcr.io/egoadmin/egoadmin-gateway` | 网关服务（内嵌前端） |
| `ghcr.io/egoadmin/egoadmin-user` | 用户服务 |
| `ghcr.io/egoadmin/egoadmin-idgen` | ID 生成服务 |

## 部署

```bash
export DOCKER_REGISTRY=ghcr.io/egoadmin
make deploy-up
```

停止：

```bash
make deploy-down
```

## Docker Compose 文件结构

部署编排：

```text
deploy/
├── docker-compose.yml         # 主入口，include 所有 compose 文件
├── .env                       # 环境变量（不提交到仓库）
├── configs/                   # 部署配置文件
│   ├── gateway/config.toml
│   ├── user/config.toml
│   └── idgen/config.toml
└── compose/
    ├── app.yml                # 应用服务：gateway、user、idgen
    ├── mysql.yml              # MySQL 数据库
    ├── redis.yml              # Redis 缓存
    ├── etcd.yml               # etcd 注册中心
    ├── minio.yml              # MinIO 对象存储
    ├── dtm.yml                # DTM 分布式事务
    ├── jaeger.yml             # Jaeger 链路追踪
    ├── meilisearch.yml        # MeiliSearch 全文搜索
    └── image-processor.yml    # 图片处理服务
```

**主 compose 文件** (`deploy/docker-compose.yml`)：

```yaml
include:
  - path: compose/mysql.yml
  - path: compose/redis.yml
  - path: compose/etcd.yml
  - path: compose/minio.yml
  - path: compose/app.yml
  # 可选组件按需启用
  # - path: compose/dtm.yml
  # - path: compose/jaeger.yml
  # - path: compose/meilisearch.yml
  # - path: compose/image-processor.yml
```

**应用服务 compose** (`deploy/compose/app.yml`)：

```yaml
services:
  gateway:
    image: ${DOCKER_REGISTRY}/egoadmin-gateway:${TAG:-latest}
    ports:
      - "9001:9001"
    depends_on:
      mysql-user:
        condition: service_healthy
      redis:
        condition: service_started
      etcd:
        condition: service_started
    environment:
      EGOADMIN_CLIENT_MYSQL_DSN: ${MYSQL_DSN}
      EGOADMIN_CLIENT_REDIS_ADDR: redis:6379
      EGOADMIN_ETCD_ADDRS: '["etcd:2379"]'

  user:
    image: ${DOCKER_REGISTRY}/egoadmin-user:${TAG:-latest}
    ports:
      - "9002:9002"
    depends_on:
      mysql-user:
        condition: service_healthy
    environment:
      EGOADMIN_CLIENT_MYSQL_DSN: ${MYSQL_DSN_USER}

  idgen:
    image: ${DOCKER_REGISTRY}/egoadmin-idgen:${TAG:-latest}
    ports:
      - "9202:9202"
    depends_on:
      mysql-idgen:
        condition: service_healthy
```

## CI 流水线

EgoAdmin CI 流水线基于 GitHub Actions，主要阶段如下。

**流水线概览**：

```text
push / PR
  → Lint (golangci-lint + eslint)
  → Unit tests (go test -race ./...)
  → Frontend build (pnpm run build)
  → Service check (make service.check)
  → Build binaries (make build)
  → Build Docker images (make image.build)
  → E2E tests (make e2e)
  → Push images (on main branch)
```

**关键 CI 步骤**：

```yaml
# .github/workflows/ci.yml 概要
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: make lint

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: go test -race ./...

  build:
    needs: [lint, test]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: make gen           # 先生成代码
      - run: make build         # 再构建
      - run: make image.build   # 构建镜像
```

::: tip
CI 中构建前必须先运行 `make gen`，确保 proto、Wire 等生成代码是最新的。
:::

## 构建命令详解

```bash
# 构建所有服务
make build

# 构建单个服务
make build SERVICE=user

# Alpine 精简镜像构建
make build.alpine SERVICE=gateway

# Alpine ARM64 构建（Apple Silicon / ARM 服务器）
make build.alpine-arm64 SERVICE=idgen
```

**构建产物**：

```text
output/
├── gateway
│   └── egoadmin-gateway       # gateway 二进制
├── user
│   └── egoadmin-user           # user 二进制
└── idgen
    └── egoadmin-idgen           # idgen 二进制
```

gateway 构建前会自动检查 `web/dist/index.html` 是否存在。前端产物内嵌到 gateway 二进制中。

## 服务健康检查

每个服务提供以下健康检查端点：

| 端点 | 说明 | 用途 |
|------|------|------|
| `GET /healthz` | 存活探针 | Kubernetes liveness probe |
| `GET /readyz` | 就绪探针 | Kubernetes readiness probe |

**手动检查**：

```bash
# 检查 gateway
curl -s http://localhost:9001/healthz
curl -s http://localhost:9001/readyz

# 检查 user gRPC 服务（通过 governor 端口）
curl -s http://localhost:9003/healthz

# 检查 idgen
curl -s http://localhost:9203/healthz
```

**Docker Compose 健康检查**：

```yaml
healthcheck:
  test: ["CMD", "wget", "-q", "--spider", "http://localhost:9001/healthz"]
  interval: 10s
  timeout: 5s
  retries: 3
  start_period: 30s
```

## 发布附件

| 文件 | 说明 |
|------|------|
| `vendor.tar.gz` | Go vendor 依赖，离线构建可用 |
| `openapi.tar.gz` | OpenAPI 接口规范 |
| `egoadmin-tools-linux-amd64.tar.gz` | protoc、buf、wire、atlas 等工具包 |

下载工具包：

```bash
curl -L https://github.com/egoadmin/egoadmin/releases/latest/download/egoadmin-tools-linux-amd64.tar.gz | tar xz
export PATH="$PWD/egoadmin-tools-linux-amd64:$PATH"
```

## 发布前检查清单

```bash
make gen
make lint
go test -race ./...
cd web && pnpm run build
make service.check SERVICE=gateway
make service.check SERVICE=user
make service.check SERVICE=idgen
make e2e E2E_TIMEOUT=20m
```

涉及数据库变更时补充：

```bash
make migrate.validate SERVICE=gateway
make migrate.validate SERVICE=user
make migrate.validate SERVICE=idgen
```

