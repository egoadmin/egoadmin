# gRPC 接口设计

IDGen 服务通过两个独立的 gRPC 服务对外暴露能力：`SegmentService` 号段分配和 `MachineLeaseService` 机器租约管理。其他业务服务通过 `internal/client/idgenclient` 客户端访问 IDGen 服务。

## 概述

IDGen 服务的 gRPC 接口分为三类：

| 接口类别 | 服务名 | 职责 |
|----------|--------|------|
| 号段分配 | `SegmentService` | 创建/分配 ID 号段，健康检查 |
| 机器租约 | `MachineLeaseService` | 分配/续约/释放进程级机器租约 |
| 健康检查 | `SegmentService.Health` | 检查号段分配器可用性 |

服务地址端口：

| 服务 | HTTP | gRPC | Governor |
|------|------|------|----------|
| idgen | 9201 | 9202 | 9203 |

::: tip 客户端使用方式
非 idgen 服务通过 `idgenclient.Client` 访问 IDGen 服务，客户端由 Wire 自动注入，使用 etcd 服务发现。
:::

## 核心用法

### Proto 服务定义

IDGen 定义了两个独立的 gRPC 服务，分别管理号段和机器租约：

```protobuf
// 号段服务：提供进程内 ID 生成器的号段分配
service SegmentService {
    // EnsureSegment 在号段不存在时创建定义
    rpc EnsureSegment(EnsureSegmentRequest) returns (EnsureSegmentResponse);
    // AllocateSegment 分配一个不重叠的半开区间 [start, end)
    rpc AllocateSegment(AllocateSegmentRequest) returns (AllocateSegmentResponse);
    // Health 检查号段分配器是否可用
    rpc Health(SegmentServiceHealthRequest) returns (SegmentServiceHealthResponse);
}

// 机器租约服务：为 ID 生成器提供进程级机器租约
service MachineLeaseService {
    // AllocateLease 为实例分配或续租机器租约
    rpc AllocateLease(AllocateLeaseRequest) returns (AllocateLeaseResponse);
    // RenewLease 续约已有机器租约
    rpc RenewLease(RenewLeaseRequest) returns (RenewLeaseResponse);
    // ReleaseLease 释放已有机器租约
    rpc ReleaseLease(ReleaseLeaseRequest) returns (ReleaseLeaseResponse);
}
```

### 接口分类详解

#### 1. SegmentService 号段服务

**EnsureSegment** -- 创建缺失的号段定义：

```protobuf
message EnsureSegmentRequest {
    string namespace   = 1;  // 命名空间，隔离部署/产品
    string name        = 2;  // 业务 ID 流名称
    int64  next_id     = 3;  // 创建时的起始 ID
    int64  step        = 4;  // 基础步长
    int64  min_step    = 5;  // 最小步长
    int64  max_step    = 6;  // 最大步长
    int32  status      = 7;  // 状态：1 启用，2 禁用
    string description = 8;  // 描述
}
```

**AllocateSegment** -- 分配号段区间：

```protobuf
message AllocateSegmentRequest {
    string namespace     = 1;  // 命名空间
    string name          = 2;  // 业务名称
    int64  requested_step = 3; // 请求的步长，服务端会按策略裁剪
}

message AllocateSegmentResponse {
    int64 start    = 1;  // 区间起始
    int64 end      = 2;  // 区间结束（不含）
    int64 step     = 3;  // 基础步长
    int64 min_step = 4;  // 最小步长
    int64 max_step = 5;  // 最大步长
    int32 status   = 6;  // 状态
}
```

#### 2. MachineLeaseService 租约服务

**AllocateLease** -- 分配机器租约：

```protobuf
message AllocateLeaseRequest {
    string namespace             = 1;  // 租约命名空间
    string instance_id           = 2;  // 实例标识
    string stable_instance_id    = 3;  // 稳定实例 ID（如 Pod 名）
    int32  max_machine_id        = 4;  // 最大机器 ID
    int64  ttl_seconds           = 5;  // 租约 TTL（秒）
    int64  renew_interval_seconds = 6; // 建议续约间隔（秒）
}

message AllocateLeaseResponse {
    string namespace             = 1;
    string instance_id           = 2;
    string session_id            = 3;  // 当前租约会话 ID
    int32  machine_id            = 4;  // 分配的机器 ID
    int64  ttl_seconds           = 5;
    int64  renew_interval_seconds = 6;
    google.protobuf.Timestamp expires_at = 7;  // 过期时间
}
```

**RenewLease** -- 续约：

```protobuf
message RenewLeaseRequest {
    string namespace             = 1;
    string instance_id           = 2;
    string session_id            = 3;
    int32  machine_id            = 4;
    int64  ttl_seconds           = 5;
    int64  renew_interval_seconds = 6;
}
```

**ReleaseLease** -- 释放：

```protobuf
message ReleaseLeaseRequest {
    string namespace   = 1;
    string instance_id = 2;
    string session_id  = 3;
    int32  machine_id  = 4;
}
```

### 客户端使用

非 idgen 服务通过 `idgenclient.Client` 访问 IDGen 服务。客户端由 EGO 的 `egrpc.Load` 构建，自动使用 etcd 服务发现：

```go
// internal/client/idgenclient/client.go
type Client struct {
    conn         *egrpc.Component
    Segment      SegmentService       // 号段操作
    MachineLease MachineLeaseService  // 租约操作
}

// 构建方式（自动服务发现）
func NewClient(_ discovery.Ready) *Client {
    conn := egrpc.Load("client.grpc.idgen").Build()
    return &Client{
        conn:         conn,
        Segment:      &segmentClient{...},
        MachineLease: &machineLeaseClient{...},
    }
}
```

客户端接口定义：

```go
type SegmentService interface {
    Ensure(ctx context.Context, namespace string, name string, cfg idgen.EnsureSegmentConfig) error
    Allocate(ctx context.Context, namespace string, name string, requestedStep int64) (idgen.Range, idgen.SegmentConfig, error)
    Health(ctx context.Context) error
}

type MachineLeaseService interface {
    Allocate(ctx context.Context, req idgen.MachineRequest) (idgen.MachineLease, error)
    Renew(ctx context.Context, lease idgen.MachineLease) error
    Release(ctx context.Context, lease idgen.MachineLease) error
}
```

### Wire 注入

在 platform/idgen/provider.go 中，客户端自动组装为 `SegmentStore` 和 `MachineAllocator`：

```go
var ProviderSet = wire.NewSet(
    NewSegmentStore,       // grpcstore.New(client) -> SegmentStore
    NewMachineAllocator,   // grpcallocator.New(client) -> MachineAllocator
    NewMachineLeaseManager,
    NewComponent,
    NewIDGetter,
)
```

调用方只需依赖 `idgen.Interface` 或 `idgen.Generator`，无需直接感知 gRPC。

## 配置示例

### 客户端配置

```toml
[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"    # 服务发现地址
debug = true                       # 启用调试日志
readTimeout = "3s"                 # 读超时
dialTimeout = "5s"                 # 连接超时
```

### 服务端 gRPC 配置

```toml
[server.grpc]
port = 9202                        # gRPC 监听端口
```

### 服务端注册

gRPC 服务在 `server/grpc_server.go` 中注册：

```go
func NewGrpcServer(opts controller.Options, _ config.EgoReady) *GrpcServer {
    s := egrpc.Load("server.grpc").Build(
        egrpc.WithUnaryInterceptor(
            grpc.Middleware(
                recovery.Recovery(),
                ecode.Ecode(),
            ),
        ),
    )

    idgenv1.RegisterSegmentServiceServer(s.Server, opts.Segment)
    idgenv1.RegisterMachineLeaseServiceServer(s.Server, opts.MachineLease)

    return &GrpcServer{Component: s}
}
```

## 实战示例

### 客户端分配号段

```go
type IDGenSegmentStore struct {
    client idgenclient.SegmentService
}

func (s *IDGenSegmentStore) Fetch(ctx context.Context, namespace, name string, step int64) (idgen.Range, idgen.SegmentConfig, error) {
    return s.client.Allocate(ctx, namespace, name, step)
}

func (s *IDGenSegmentStore) Ensure(ctx context.Context, namespace, name string, cfg idgen.EnsureSegmentConfig) error {
    return s.client.Ensure(ctx, namespace, name, cfg)
}
```

### 客户端管理机器租约

```go
type IDGenMachineAllocator struct {
    client idgenclient.MachineLeaseService
}

func (a *IDGenMachineAllocator) Allocate(ctx context.Context, req idgen.MachineRequest) (idgen.MachineLease, error) {
    return a.client.Allocate(ctx, req)
}

func (a *IDGenMachineAllocator) Renew(ctx context.Context, lease idgen.MachineLease) error {
    return a.client.Renew(ctx, lease)
}

func (a *IDGenMachineAllocator) Release(ctx context.Context, lease idgen.MachineLease) error {
    return a.client.Release(ctx, lease)
}
```

### 追踪上下文传播

客户端自动传播追踪元数据（B3、W3C Trace Context 等），通过 `outgoingContext` 从入站元数据中提取并注入到出站请求：

```go
var forwardedMetadataKeys = map[string]struct{}{
    "authorization":     {},
    "x-request-id":      {},
    "x-correlation-id":  {},
    "x-b3-traceid":      {},
    "x-b3-spanid":       {},
    "traceparent":       {},
    "tracestate":        {},
    // ... 更多追踪头
}
```

## 工作原理

### gRPC 调用链路

```text
业务服务（gateway/user）
    |
    v
idgenclient.Client（egrpc.Load("client.grpc.idgen")）
    |
    v
etcd 服务发现（etcd:///egoadmin-idgen）
    |
    v
IDGen 服务 gRPC Server（port 9202）
    |
    v
controller.SegmentGRPC / controller.MachineLeaseGRPC
    |
    v
application.SegmentUseCase / application.MachineLeaseUseCase
    |
    v
mysql.SegmentRepository / mysql.MachineLeaseRepository
    |
    v
MySQL（idgen_segment / idgen_machine_lease 表）
```

### 错误码映射

gRPC 控制器将内部错误映射为标准 gRPC 状态码：

| 内部错误 | gRPC 状态码 | 说明 |
|----------|-------------|------|
| `ErrInvalidConfig` | `InvalidArgument` | 配置参数无效 |
| `ErrNameNotFound` | `NotFound` | 号段名称不存在 |
| `ErrNameDisabled` | `FailedPrecondition` | 号段已禁用 |
| `ErrMachineLeaseLost` | `FailedPrecondition` | 机器租约丢失 |
| `ErrMachineIDOverflow` | `ResourceExhausted` | 超过最大机器 ID |
| `ErrStoreUnavailable` | `Unavailable` | 存储不可用 |
| `ErrSegmentConflict` | `Unavailable` | 号段更新冲突 |
| `context.Canceled` | `Canceled` | 请求被取消 |
| `context.DeadlineExceeded` | `DeadlineExceeded` | 请求超时 |
| 其他错误 | `Internal` | 内部错误 |

客户端 `normalizeError` 将 gRPC 状态码反向映射回内部错误，确保调用方使用统一的错误处理：

```go
switch st.Code() {
case codes.InvalidArgument:
    return fmt.Errorf("%w: %s", idgen.ErrInvalidConfig, st.Message())
case codes.NotFound:
    return idgen.ErrNameNotFound
case codes.FailedPrecondition:
    if strings.Contains(st.Message(), "lease") {
        return idgen.ErrMachineLeaseLost
    }
    return idgen.ErrNameDisabled
case codes.ResourceExhausted:
    return idgen.ErrMachineIDOverflow
case codes.Unavailable, codes.DeadlineExceeded:
    return fmt.Errorf("%w: %s", idgen.ErrStoreUnavailable, st.Message())
}
```

### 幂等性

- **AllocateLease**：对同一 `instanceID` 的重复调用返回已有租约（如存在未过期的实例租约则复用）。
- **EnsureSegment**：使用 `ON CONFLICT DO NOTHING` 语义，已存在时无副作用。
- **RenewLease**：校验 `sessionID` 和 `machineID`，不匹配则返回 `ErrMachineLeaseLost`。
- **ReleaseLease**：校验四元组 `(namespace, machineID, instanceID, sessionID)`，不匹配则返回错误。

### 超时与重试策略

| 操作 | 推荐超时 | 重试策略 |
|------|----------|----------|
| `AllocateSegment` | `fetchTimeout`（2s） | 不重试（避免 ID 冲突） |
| `AllocateLease` | `renewTimeout`（5s） | 指数退避重试（`reallocateBackoff` 2s） |
| `RenewLease` | `renewTimeout`（5s） | 最多重试直到 TTL 过期 |
| `Health` | 1s | 不重试 |

::: warning 不要重试号段分配
`AllocateSegment` 返回的区间已被消耗。如果客户端重试但原始请求实际成功，会导致区间泄漏（ID 空洞）。号段预取机制在进程内处理重试。
:::

### gRPC 反射调试

IDGen 服务端启用了 gRPC 反射，可使用 `grpcurl` 调试：

```bash
# 列出所有服务
grpcurl -plaintext 127.0.0.1:9202 list

# 列出服务方法
grpcurl -plaintext 127.0.0.1:9202 describe idgen.v1.SegmentService

# 健康检查
grpcurl -plaintext 127.0.0.1:9202 idgen.v1.SegmentService.Health

# 分配号段
grpcurl -plaintext -d '{"namespace":"egoadmin-local","name":"user_id","requested_step":100000}' \
    127.0.0.1:9202 idgen.v1.SegmentService.AllocateSegment
```

### 数据库表结构

号段存储表 `idgen_segment`：

| 列名 | 类型 | 说明 |
|------|------|------|
| `namespace` | VARCHAR(64) PK | 命名空间 |
| `name` | VARCHAR(128) PK | 业务名称 |
| `next_id` | BIGINT | 下一个可分配 ID |
| `step` | BIGINT | 基础步长 |
| `min_step` | BIGINT | 最小步长 |
| `max_step` | BIGINT | 最大步长 |
| `status` | INT | 状态：1 启用，2 禁用 |
| `last_step` | BIGINT | 上次实际领取步长 |
| `last_fetch_at` | DATETIME | 上次领取号段时间 |

机器租约表 `idgen_machine_lease`：

| 列名 | 类型 | 说明 |
|------|------|------|
| `namespace` | VARCHAR(64) PK | 租约命名空间 |
| `machine_id` | INT PK | 机器号 |
| `instance_id` | VARCHAR(255) | 实例 ID |
| `session_id` | VARCHAR(64) | 租约会话 ID |
| `ttl_millis` | BIGINT | 租约 TTL 毫秒 |
| `renew_millis` | BIGINT | 续约间隔毫秒 |
| `expires_at` | DATETIME | 过期时间 |
| `last_renewed_at` | DATETIME | 最近续约时间 |

### 过期租约清理

IDGen 服务端通过定时任务（`cron.idgen.machine.cleanup`）清理过期租约：

```go
// 默认保留 7 天，每次最多清理 1000 条
const defaultMachineCleanupRetention = 7 * 24 * time.Hour
const defaultMachineCleanupLimit = 1000
```

## 常见问题

### 连接失败

**症状**：`ErrStoreUnavailable` 或 gRPC `Unavailable` 错误。

**排查**：

- IDGen 服务是否已启动（端口 9202）。
- etcd 是否可达，服务是否已注册。
- `dialTimeout` 是否过短。

```bash
# 检查 etcd 中的服务注册
etcdctl get /egoadmin/ --prefix | grep idgen

# 直接测试 gRPC 连接
grpcurl -plaintext 127.0.0.1:9202 idgen.v1.SegmentService.Health
```

### 号段分配冲突

**症状**：`ErrSegmentConflict`。

**原因**：多个 IDGen 实例同时更新同一号段行。GORM 使用 `SELECT ... FOR UPDATE` + `WHERE next_id = ?` 的乐观锁模式，`RowsAffected != 1` 时返回冲突。

**解决**：重试即可，号段分配器在进程内自动重试。

### 租约续约失败

**症状**：`ErrMachineLeaseLost`。

**排查**：

- Redis 或 MySQL 连接是否正常。
- 租约是否已被其他进程抢占（`sessionID` 不匹配）。
- 后台 cron 任务 `cron.idgen.machine.renew` 是否正常运行。
- `renewTimeout` 是否足够（默认 5 秒）。

### 客户端超时

**症状**：`context.DeadlineExceeded`。

**排查**：

- `readTimeout` 配置是否合理（推荐 3 秒）。
- 数据库是否响应缓慢。
- 号段拉取延迟是否异常（检查 `idgen_segment_fetch_seconds` 指标）。

### JS 精度丢失

::: warning
gRPC 返回的 `int64` 在 JSON 序列化时可能丢失精度。使用 idcodec 编码为字符串，或将 ID 字段定义为 `string` 类型。
:::

## 参考链接

- `api/proto-internal/idgen/v1/idgen.proto` -- Proto 服务和消息定义
- `api/gen/go/idgen/v1/idgen_grpc.pb.go` -- 生成的 gRPC 桩代码
- `internal/app/idgen/controller/segment_grpc.go` -- 号段 gRPC 控制器
- `internal/app/idgen/controller/machine_grpc.go` -- 机器租约 gRPC 控制器
- `internal/app/idgen/controller/errors.go` -- 错误码映射
- `internal/app/idgen/server/grpc_server.go` -- gRPC 服务端构建与注册
- `internal/client/idgenclient/client.go` -- gRPC 客户端实现
- `internal/component/idgen/store/grpcstore/store.go` -- 号段 gRPC 存储适配器
- `internal/component/idgen/machine/grpcallocator/allocator.go` -- 机器租约 gRPC 分配器
