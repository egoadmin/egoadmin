# Testing & Deployment

## Test Layers

| Layer | Command | Purpose |
|-------|---------|---------|
| unit tests | `go test ./...` | domain, application, helpers |
| race tests | `go test -race ./...` | concurrency safety |
| service check | `make service.check SERVICE=user` | layout, providers, registration, health |
| frontend type check | `cd web && pnpm run type-check` | TypeScript |
| frontend build | `cd web && pnpm run build` | build and permission contract |
| e2e | `make e2e E2E_TIMEOUT=20m` | gateway user-visible flows |

## Unit Test Patterns with testify

EgoAdmin uses `testify` as the testing framework. Test files live in the same package directory as the source code.

**Basic structure**:

```go
package application

import (
  "testing"

  "github.com/stretchr/testify/assert"
  "github.com/stretchr/testify/require"
)

func TestUserUseCase_CreateUser(t *testing.T) {
  // assert for non-fatal assertions
  assert.Equal(t, expected, actual)

  // require for fatal assertions, stops immediately on failure
  require.NoError(t, err)
}
```

::: tip
`require` calls `t.FailNow()` on failure, `assert` continues execution. Use `require` for preconditions (like error checks) and `assert` for result validation.
:::

**Using mocks**:

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

## Table-Driven Tests

Table-driven tests are the idiomatic Go pattern. EgoAdmin recommends using them in all tests:

```go
func TestCalculateDiscount(t *testing.T) {
  tests := []struct {
    name    string
    amount  float64
    level   int
    want    float64
    wantErr bool
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
      name:   "zero amount is valid",
      amount: 0,
      level:  0,
      want:   0,
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

**Testing database layer**:

```go
func TestUserRepository_Create(t *testing.T) {
  db := setupTestDB(t) // use testcontainers or in-memory SQLite
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

## Integration Tests

Integration tests require real middleware (MySQL, Redis). Use testcontainers or the running dev environment.

**Using dev environment**:

```bash
# Start development middleware
make dev-up

# Run integration tests
go test -tags=integration ./internal/app/user/...

# Cleanup
make dev-down
```

**Using testcontainers**:

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
  // Start MySQL test container
  // Return *sql.DB connection
  // Container auto-cleaned when test ends
  // ...
}

func TestUserMySQLRepository_Integration(t *testing.T) {
  if testing.Short() {
    t.Skip("skipping integration test in short mode")
  }

  db := setupMySQLContainer(t)
  defer db.Close()

  repo := NewUserMySQLRepository(db)
  // Run tests...
}
```

## E2E Test Framework

The E2E tests live at `test/e2e/gateway/` and test the full request flow through the gateway.

**Test structure**:

```text
test/e2e/gateway/
├── main_test.go          # Test entry point and setup
├── auth_test.go          # Login/logout/refresh token
├── user_test.go          # User CRUD
├── role_test.go          # Role permissions
├── department_test.go    # Department tree
├── upload_test.go        # Upload/CDN
└── permission_test.go    # Permission deny / data permissions
```

**Topology**:

```text
test client
  -> gateway HTTP compatibility
  -> user gRPC via etcd
  -> service DB/Redis
```

Scenarios covered:

- Login / logout.
- User management.
- Role permissions.
- Department tree.
- Upload / CDN.
- Permission deny.
- Data permissions.

**Running E2E tests**:

```bash
# Run all e2e tests (services must be running)
make e2e E2E_TIMEOUT=20m

# Custom timeout
make e2e E2E_TIMEOUT=30m
```

::: warning
E2E tests require all three services (gateway, user, idgen) and middleware (MySQL, Redis, etcd) to be running. Use `make deploy-up` or `make dev-up` + `make run`.
:::

**E2E test example**:

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

  // Login
  resp, err := http.PostForm(baseURL+"/api/auth/login", map[string][]string{
    "username": {"admin"},
    "password": {"123456"},
  })
  require.NoError(t, err)
  defer resp.Body.Close()
  assert.Equal(t, http.StatusOK, resp.StatusCode)

  // Parse token
  var loginResp LoginResponse
  decodeJSON(resp.Body, &loginResp)
  require.NotEmpty(t, loginResp.AccessToken)

  // Logout
  req, _ := http.NewRequest("POST", baseURL+"/api/auth/logout", nil)
  req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
  logoutResp, err := http.DefaultClient.Do(req)
  require.NoError(t, err)
  defer logoutResp.Body.Close()
  assert.Equal(t, http.StatusOK, logoutResp.StatusCode)
}
```

## Service Check

The service check validates service structure, providers, registration, and health endpoints:

```bash
make service.check SERVICE=gateway
make service.check SERVICE=user
make service.check SERVICE=idgen
```

Checks include:

- `cmd/<service>/Dockerfile`
- `internal/app/<service>/server`
- `controller`
- `application`
- `domain`
- `adapter`
- `schema`
- `atlas/migrations/<service>/atlas.sum`
- gRPC service registration
- HTTP `/healthz` and `/readyz`
- ProviderSet
- Architecture dependency constraints

## Build

```bash
make build
make build SERVICE=user
make build.alpine SERVICE=gateway
make build.alpine-arm64 SERVICE=idgen
```

**Build artifacts**:

```text
output/
├── gateway
│   └── egoadmin-gateway       # gateway binary
├── user
│   └── egoadmin-user           # user binary
└── idgen
    └── egoadmin-idgen           # idgen binary
```

The gateway build automatically checks that `web/dist/index.html` exists. Frontend assets are embedded into the gateway binary.

## Images

```bash
make image.build
make image.build SERVICE=user
```

Published images:

| Image | Description |
|-------|-------------|
| `ghcr.io/egoadmin/egoadmin-gateway` | gateway with embedded frontend |
| `ghcr.io/egoadmin/egoadmin-user` | user service |
| `ghcr.io/egoadmin/egoadmin-idgen` | ID generation service |

## Deployment

```bash
export DOCKER_REGISTRY=ghcr.io/egoadmin
make deploy-up
```

Stop:

```bash
make deploy-down
```

## Docker Compose File Structure

Deployment orchestration:

```text
deploy/
├── docker-compose.yml         # Main entry, includes all compose files
├── .env                       # Environment variables (not committed)
├── configs/                   # Deployment config files
│   ├── gateway/config.toml
│   ├── user/config.toml
│   └── idgen/config.toml
└── compose/
    ├── app.yml                # Application services: gateway, user, idgen
    ├── mysql.yml              # MySQL database
    ├── redis.yml              # Redis cache
    ├── etcd.yml               # etcd registry
    ├── minio.yml              # MinIO object storage
    ├── dtm.yml                # DTM distributed transactions
    ├── jaeger.yml             # Jaeger tracing
    ├── meilisearch.yml        # MeiliSearch full-text search
    └── image-processor.yml    # Image processing service
```

**Main compose file** (`deploy/docker-compose.yml`):

```yaml
include:
  - path: compose/mysql.yml
  - path: compose/redis.yml
  - path: compose/etcd.yml
  - path: compose/minio.yml
  - path: compose/app.yml
  # Optional components, enable as needed
  # - path: compose/dtm.yml
  # - path: compose/jaeger.yml
  # - path: compose/meilisearch.yml
  # - path: compose/image-processor.yml
```

**Application compose** (`deploy/compose/app.yml`):

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

## CI Pipeline

The EgoAdmin CI pipeline is based on GitHub Actions with the following main stages.

**Pipeline overview**:

```text
push / PR
  -> Lint (golangci-lint + eslint)
  -> Unit tests (go test -race ./...)
  -> Frontend build (pnpm run build)
  -> Service check (make service.check)
  -> Build binaries (make build)
  -> Build Docker images (make image.build)
  -> E2E tests (make e2e)
  -> Push images (on main branch)
```

**Key CI steps**:

```yaml
# .github/workflows/ci.yml summary
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
      - run: make gen           # Generate code first
      - run: make build         # Then build
      - run: make image.build   # Build images
```

::: tip
In CI, always run `make gen` before building to ensure proto, Wire, and other generated code is up to date.
:::

## Service Health Checks

Each service provides the following health check endpoints:

| Endpoint | Description | Usage |
|----------|-------------|-------|
| `GET /healthz` | Liveness probe | Kubernetes liveness probe |
| `GET /readyz` | Readiness probe | Kubernetes readiness probe |

**Manual checks**:

```bash
# Check gateway
curl -s http://localhost:9001/healthz
curl -s http://localhost:9001/readyz

# Check user gRPC service (via governor port)
curl -s http://localhost:9003/healthz

# Check idgen
curl -s http://localhost:9203/healthz
```

**Docker Compose health check**:

```yaml
healthcheck:
  test: ["CMD", "wget", "-q", "--spider", "http://localhost:9001/healthz"]
  interval: 10s
  timeout: 5s
  retries: 3
  start_period: 30s
```

## Release Checklist

```bash
make service.check SERVICE=gateway
make service.check SERVICE=user
make service.check SERVICE=idgen
```

