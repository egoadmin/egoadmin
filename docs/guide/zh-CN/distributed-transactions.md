# 分布式事务（DTM）

EgoAdmin 集成 DTM 处理跨服务写一致性。单服务写入优先使用本地数据库事务，只有跨服务写入确实需要一致性协调时才使用 DTM。

## 模式选择

| 场景 | 推荐模式 |
|------|----------|
| 单服务单库写入 | 本地事务 |
| 单服务多表写入 | 本地事务 |
| 跨服务写入，可补偿 | Saga |
| 跨服务写入，需要预留/确认/取消 | TCC |
| 业务写入后可靠发送消息 | 事务消息 |

::: warning
不要为了“看起来更分布式”而使用 DTM。DTM 会增加 branch API、补偿逻辑、屏障表和测试复杂度。
:::

## 角色模型

| 角色 | 含义 | EgoAdmin 中的位置 |
|------|------|------------------|
| AP | 应用程序，发起全局事务 | `application` 层 |
| TM | 事务管理器 | DTM 服务 |
| RM | 资源管理器，执行本地分支 | user/idgen 等业务服务 |

## 配置

DTM server 和 branch service target 分开配置：

```toml
[component.dtm]
enabled = true
server = "etcd:///egoadmin-dtm"

[component.dtm.branch.user]
target = "etcd:///egoadmin-user"

[component.dtm.branch.idgen]
target = "etcd:///egoadmin-idgen"
```

规则：

- DTM server 是事务协调器地址。
- branch target 是业务服务地址。
- branch URL 用完整 gRPC 方法名拼接。
- 不从 `ClientConn.Target()` 或 `egrpc.Load()` 推断 branch URL。

## DTM Server 部署

DTM 作为独立服务运行，EgoAdmin 通过 `dtmdriver-ego` 插件使用 etcd 注册发现。

```toml
# deploy/compose/dtm.yml
services:
  dtm:
    image: ghcr.io/dtm-labs/dtm:latest
    command: ["dtm", "-c", "/etc/dtm/config.yaml"]
    volumes:
      - ./dtm-config.yaml:/etc/dtm/config.yaml:ro
    ports:
      - "36789:36789"   # HTTP API
      - "36790:36790"   # gRPC
```

DTM 服务端配置示例：

```yaml
# dtm-config.yaml
Store:
  Driver: "mysql"
  Host: "mysql-dtm"
  Port: 3306
  User: "dtm"
  Password: "${DTM_DB_PASSWORD}"
  Database: "dtm"

MicroService:
  Driver: "dtm-driver-ego"
  EgoTarget: "etcd://etcd:2379/dtm-driver-ego"
```

::: tip
生产环境 DTM 需要独立数据库。不要与业务库共用，避免单点故障。
:::

在应用中注册 DTM 驱动：

```go
import (
  _ "github.com/dtm-labs/dtmdriver-ego"
  _ "github.com/dtm-labs/client/dtmgrpc"
)

func init() {
  // dtmdriver-ego 会在启动时自动注册 etcd 驱动
  // 无需额外调用 setup
}
```

## 完整 Saga 示例：创建订单

Saga 适用于可补偿的跨服务流程。以下示例展示 "创建订单 → 预留库存 → 扣款" 的完整编排。

**业务编排层** (`application/order_usecase.go`)：

```go
package application

import (
  "context"

  "github.com/dtm-labs/client/dtmgrpc"
  orderpb "github.com/egoadmin/egoadmin/api/order/v1"
  stockpb "github.com/egoadmin/egoadmin/api/stock/v1"
  paymentpb "github.com/egoadmin/egoadmin/api/payment/v1"
)

type OrderUseCase struct {
  dtmServer   string
  stockTarget string
  payTarget   string
}

func (uc *OrderUseCase) CreateOrder(ctx context.Context, req *CreateOrderReq) error {
  gid := dtmgrpc.MustGenGid(uc.dtmServer)

  saga := dtmgrpc.NewSagaGrpc(uc.dtmServer, gid).
    Add(
      // forward: 预留库存
      uc.stockTarget+"/stock.v1.StockService/Reserve",
      // compensate: 释放库存
      uc.stockTarget+"/stock.v1.StockService/Release",
      &stockpb.ReserveRequest{
        OrderId: req.OrderID,
        Items:   toStockItems(req.Items),
      },
    ).
    Add(
      // forward: 扣款
      uc.payTarget+"/payment.v1.PaymentService/Deduct",
      // compensate: 退款
      uc.payTarget+"/payment.v1.PaymentService/Refund",
      &paymentpb.DeductRequest{
        OrderId: req.OrderID,
        UserId:  req.UserID,
        Amount:  req.TotalAmount,
      },
    )

  // 本地创建订单记录（状态为 pending）
  if err := uc.orderRepo.CreatePending(ctx, req); err != nil {
    return err
  }

  // 提交全局事务
  return saga.Submit()
}
```

**Branch Handler — 库存服务** (`controller/stock_dtm_grpc.go`)：

```go
package controller

import (
  "context"
  "database/sql"

  "github.com/dtm-labs/client/dtmgrpc"
  pb "github.com/egoadmin/egoadmin/api/stock/v1"
)

type StockDtmController struct {
  pb.UnimplementedStockServiceServer
  db *sql.DB
}

func (c *StockDtmController) Reserve(ctx context.Context, req *pb.ReserveRequest) (*pb.ReserveResponse, error) {
  barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
  if err != nil {
    return nil, err
  }

  err = barrier.CallWithDB(c.db, func(tx *sql.Tx) error {
    for _, item := range req.GetItems() {
      if err := deductStock(ctx, tx, item.ProductId, item.Quantity); err != nil {
        return err
      }
    }
    return nil
  })
  if err != nil {
    return nil, err
  }
  return &pb.ReserveResponse{}, nil
}

func (c *StockDtmController) Release(ctx context.Context, req *pb.ReserveRequest) (*pb.ReserveResponse, error) {
  barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
  if err != nil {
    return nil, err
  }

  err = barrier.CallWithDB(c.db, func(tx *sql.Tx) error {
    for _, item := range req.GetItems() {
      if err := addStock(ctx, tx, item.ProductId, item.Quantity); err != nil {
        return err
      }
    }
    return nil
  })
  if err != nil {
    return nil, err
  }
  return &pb.ReserveResponse{}, nil
}
```

## TCC 模式

TCC（Try / Confirm / Cancel）适用于需要资源预留的场景。与 Saga 相比，TCC 在 Try 阶段仅预留资源而非直接扣减。

**Proto 定义**：

```protobuf
service PaymentTccService {
  rpc Try(TryDeductRequest) returns (TryDeductResponse);
  rpc Confirm(TryDeductRequest) returns (TryDeductResponse);
  rpc Cancel(TryDeductRequest) returns (TryDeductResponse);
}
```

**编排层**：

```go
func (uc *OrderUseCase) CreateOrderTCC(ctx context.Context, req *CreateOrderReq) error {
  gid := dtmgrpc.MustGenGid(uc.dtmServer)

  tcc := dtmgrpc.NewTccGrpc(uc.dtmServer, gid)
  err := tcc.CallBranch(
    &paymentpb.TryDeductRequest{
      OrderId: req.OrderID,
      Amount:  req.TotalAmount,
    },
    uc.payTarget+"/payment.v1.PaymentTccService/Try",
    uc.payTarget+"/payment.v1.PaymentTccService/Confirm",
    uc.payTarget+"/payment.v1.PaymentTccService/Cancel",
  )
  if err != nil {
    return err
  }

  return tcc.Submit()
}
```

**TCC Handler — 支付服务**：

```go
package controller

import (
  "context"
  "database/sql"
  "errors"

  "github.com/dtm-labs/client/dtmgrpc"
  pb "github.com/egoadmin/egoadmin/api/payment/v1"
)

type PaymentTccController struct {
  pb.UnimplementedPaymentTccServiceServer
  db *sql.DB
}

// Try: 冻结金额（预留资源）
func (c *PaymentTccController) Try(ctx context.Context, req *pb.TryDeductRequest) (*pb.TryDeductResponse, error) {
  barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
  if err != nil {
    return nil, err
  }

  err = barrier.CallWithDB(c.db, func(tx *sql.Tx) error {
    // 检查余额
    balance, err := queryBalance(ctx, tx, req.GetUserId())
    if err != nil {
      return err
    }
    if balance < req.GetAmount() {
      return errors.New("insufficient balance")
    }
    // 冻结金额
    return freezeAmount(ctx, tx, req.GetOrderId(), req.GetAmount())
  })
  if err != nil {
    return nil, err
  }
  return &pb.TryDeductResponse{}, nil
}

// Confirm: 确认扣减（冻结金额变为实际扣除）
func (c *PaymentTccController) Confirm(ctx context.Context, req *pb.TryDeductRequest) (*pb.TryDeductResponse, error) {
  barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
  if err != nil {
    return nil, err
  }

  err = barrier.CallWithDB(c.db, func(tx *sql.Tx) error {
    return confirmDeduct(ctx, tx, req.GetOrderId())
  })
  if err != nil {
    return nil, err
  }
  return &pb.TryDeductResponse{}, nil
}

// Cancel: 取消冻结（释放预留资源）
func (c *PaymentTccController) Cancel(ctx context.Context, req *pb.TryDeductRequest) (*pb.TryDeductResponse, error) {
  barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
  if err != nil {
    return nil, err
  }

  err = barrier.CallWithDB(c.db, func(tx *sql.Tx) error {
    return unfreezeAmount(ctx, tx, req.GetOrderId())
  })
  if err != nil {
    return nil, err
  }
  return &pb.TryDeductResponse{}, nil
}
```

::: warning
TCC 的 Try 必须是幂等的且可空回滚。Cancel 到达时 Try 可能尚未执行，此时应直接返回成功（空补偿）。参见 Barrier 屏障章节。
:::

## 事务消息模式

适用于 "先完成本地事务，再可靠发送消息" 的场景。DTM 保证消息最终投递。

```go
func (uc *OrderUseCase) CreateOrderWithMessage(ctx context.Context, req *CreateOrderReq) error {
  gid := dtmgrpc.MustGenGid(uc.dtmServer)

  msg := dtmgrpc.NewMsgGrpc(uc.dtmServer, gid)
  msg.Add(
    uc.notifyTarget+"/notification.v1.NotificationService/SendOrderCreated",
    &notifpb.OrderCreatedRequest{
      OrderId: req.OrderID,
      UserId:  req.UserID,
    },
  )

  // 本地事务：创建订单
  err := uc.localTransaction(ctx, func(tx *sql.Tx) error {
    if err := insertOrder(ctx, tx, req); err != nil {
      return err
    }
    // 把 DTM barrier 操作放入同一本地事务
    return msg.Prepare(tx)
  })
  if err != nil {
    return err
  }

  return msg.Submit()
}
```

::: tip
事务消息的 Prepare 阶段必须与业务操作在同一本地事务中提交。DTM 通过 barrier 表记录 prepare 状态，保证消息不丢。
:::

## Barrier 屏障

参与 DTM 本地分支的 RM 数据库需要 `dtm_barrier` 表。屏障解决：

- 重复请求的等幂。
- 空补偿。
- 悬挂。

典型伪代码：

```go
func (c *UserDtmController) FreezeBalance(ctx context.Context, req *pb.FreezeBalanceRequest) (*pb.FreezeBalanceResponse, error) {
  barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
  if err != nil {
    return nil, err
  }

  err = barrier.CallWithDB(c.db, func(tx *sql.Tx) error {
    return c.balanceRepo.Freeze(ctx, tx, req.GetUserId(), req.GetAmount())
  })
  if err != nil {
    return nil, err
  }

  return &pb.FreezeBalanceResponse{}, nil
}
```

### Barrier 表结构

每个参与本地事务的 RM 数据库都需要创建 `dtm_barrier` 表：

```sql
CREATE TABLE IF NOT EXISTS dtm_barrier (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  gid        VARCHAR(128) NOT NULL COMMENT '全局事务 ID',
  branch_id  VARCHAR(128) NOT NULL COMMENT '分支 ID',
  op         VARCHAR(45)  NOT NULL COMMENT '操作类型: action/compensate/rollback',
  barrier_id VARCHAR(45)  NOT NULL COMMENT '屏障 ID，防止悬挂',
  reason     VARCHAR(45)  NOT NULL COMMENT '创建原因',
  create_time TIMESTAMP   DEFAULT CURRENT_TIMESTAMP,
  update_time TIMESTAMP   DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY gid_barrier (gid, branch_id, op, barrier_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

::: warning
生产环境必须在所有参与 DTM 分支的数据库中创建此表。遗漏 barrier 表会导致幂等失败。
:::

## 补偿处理模式

补偿 handler 是 Saga 模式的关键。编写补偿逻辑时需遵循以下原则。

**基本要求**：

| 要求 | 说明 |
|------|------|
| 幂等 | 相同参数多次调用结果相同 |
| 可空回滚 | 前进操作未执行时，补偿直接成功 |
| 尽力回滚 | 补偿不能抛出不可恢复的错误 |

**补偿函数示例**：

```go
// ReleaseStock 是 ReserveStock 的补偿操作
func (c *StockDtmController) ReleaseStock(ctx context.Context, req *pb.ReserveRequest) (*pb.ReserveResponse, error) {
  barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
  if err != nil {
    return nil, err
  }

  err = barrier.CallWithDB(c.db, func(tx *sql.Tx) error {
    // 查询预留记录
    reserved, err := queryReservedStock(ctx, tx, req.GetOrderId())
    if err != nil {
      return err
    }
    // 空补偿：如果预留记录不存在，说明 forward 未执行，直接返回成功
    if reserved == nil {
      return nil
    }
    // 恢复库存
    for _, item := range reserved.Items {
      if err := addStock(ctx, tx, item.ProductId, item.Quantity); err != nil {
        return err
      }
    }
    // 删除预留记录（标记已补偿）
    return markCompensated(ctx, tx, req.GetOrderId())
  })
  if err != nil {
    return nil, err
  }
  return &pb.ReserveResponse{}, nil
}
```

**补偿失败重试策略**：

```go
// DTM 默认会对失败的补偿进行重试。可通过 saga options 配置重试间隔和最大重试次数。
saga := dtmgrpc.NewSagaGrpc(uc.dtmServer, gid).
  Add(forward, compensate, data).
  SetOptions(map[string]string{
    "RetryLimit":   "3",
    "TimeoutToFail": "30",
    "BranchTimeout": "10",
  })
```

## 跨服务 gRPC 通信

DTM branch 之间通过 gRPC 进行通信。EgoAdmin 使用 etcd 作为服务发现后端。

**Client 配置**：

```toml
[client.grpc.stock]
addr = "etcd:///egoadmin-stock"
readTimeout = "5s"
dialTimeout = "3s"

[client.grpc.payment]
addr = "etcd:///egoadmin-payment"
readTimeout = "5s"
dialTimeout = "3s"
```

**Branch Target 解析规则**：

branch target 地址从配置文件读取，不从 gRPC client 连接推断：

```go
type OrderUseCase struct {
  dtmServer   string // [component.dtm.server]
  stockTarget string // [component.dtm.branch.stock.target]
  payTarget   string // [component.dtm.branch.payment.target]
}
```

branch URL 由 target + 完整 gRPC 方法路径拼接：

```go
// 正确：使用 branch target 拼接
url := uc.stockTarget + "/stock.v1.StockService/Reserve"

// 错误：不要从 ClientConn 推断
// url := stockConn.Target() + "/stock.v1.StockService/Reserve"  // ✗
```

::: warning
绝对不要从 `ClientConn.Target()` 或 `egrpc.Load()` 推断 branch URL。服务发现结果可能与预期不同。
:::

## 常见问题与排查

### DTM Server 连接失败

```text
现象：Submit() 返回 connection refused
排查：
1. 确认 DTM 服务已启动且端口可达
2. 检查 [component.dtm.server] 配置
3. etcd 中是否有 DTM 注册信息：etcdctl get /dtm/ --prefix
4. 容器内确认 DNS 解析正确
```

### Branch 执行超时

```text
现象：全局事务执行超时，branch 未完成
排查：
1. 检查 branch handler 是否阻塞（数据库锁、慢查询）
2. 调大 BranchTimeout 配置
3. 检查 gRPC 超时是否合理（readTimeout）
4. 查看 DTM 服务日志确认重试次数
```

### 幂等失败

```text
现象：相同请求重复执行结果不一致
排查：
1. 确认 dtm_barrier 表已创建且索引正确
2. handler 是否正确使用 barrier.CallWithDB()
3. 业务操作是否在 barrier.CallWithDB 回调内完成
4. 检查 barrier 表是否有异常记录
```

### 空补偿失败

```text
现象：Cancel/Compensate 返回 "空补偿失败"
排查：
1. handler 是否检查了业务记录是否存在
2. 记录不存在时应返回 nil（成功），而非 error
3. 检查 barrier 表是否有 forward 记录
```

### 悬挂防护

```text
现象：DTM 报告 "悬挂" 错误
说明：Cancel 先于 Try 到达，barrier 表记录后 Try 再到达时被拒绝
处理：这是正确行为。检查为什么 Try 会如此晚到达，考虑优化超时时间
```

### DTM 状态查询

```bash
# 查看全局事务状态
curl http://localhost:36789/api/dtmsvr/query?gid=<global-id>

# 通过 DTM 管理界面查看所有事务
# 默认地址：http://localhost:36789
```

## 目录建议

```text
internal/app/user/
├── controller/
│   └── dtm_grpc.go             # branch gRPC controller
├── application/
│   └── transfer_usecase.go     # 全局事务编排（发起方）
├── domain/
│   └── balance/
└── adapter/persistence/mysql/
    └── balance_repository.go   # branch 本地事务修改
```

## 测试场景

| 场景 | 必须覆盖 |
|------|----------|
| 全部分支成功 | 全局事务成功 |
| 第一分支成功第二分支失败 | 第一分支补偿 |
| 补偿重复到达 | 幂等 |
| Try 未到达，Cancel 先到达 | 空补偿 |
| Cancel 到达后 Try 又到达 | 悬挂防护 |
| DTM 不可用 | 发起方返回明确错误 |
| branch 服务不可用 | DTM 重试或失败路径可观测 |

## 验证命令

```bash
make dev-up
make run
go test -race ./...
make e2e E2E_TIMEOUT=20m
```

