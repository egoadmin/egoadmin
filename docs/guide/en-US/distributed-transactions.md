# Distributed Transactions (DTM)

EgoAdmin integrates DTM for cross-service write consistency. Use local database transactions for single-service writes. DTM should only be used when cross-service writes truly need consistency coordination.

::: warning
Do not use DTM just to "look more distributed." DTM adds branch APIs, compensation logic, barrier tables, and testing complexity.
:::

## Role Model

| Role | Meaning | EgoAdmin Location |
|------|---------|-------------------|
| AP | Application, initiates global transaction | `application` layer |
| TM | Transaction Manager | DTM service |
| RM | Resource Manager, executes local branches | user/idgen and other business services |

## Pattern Selection

| Scenario | Recommended Pattern |
|----------|---------------------|
| single service, single database | local transaction |
| single service, multiple tables | local transaction |
| cross-service writes with compensation | Saga |
| reserve/confirm/cancel workflow | TCC |
| reliable message after local write | transactional message |

::: warning
Do not use DTM when a local transaction suffices. DTM adds branch APIs, compensation logic, barrier tables, and testing overhead.
:::

## Config

DTM server and branch service targets are configured separately:

```toml
[component.dtm]
enabled = true
server = "etcd:///egoadmin-dtm"

[component.dtm.branch.user]
target = "etcd:///egoadmin-user"

[component.dtm.branch.idgen]
target = "etcd:///egoadmin-idgen"
```

Rules:

- DTM server is the transaction coordinator address.
- Branch target is the business service address.
- Branch URLs are assembled from the full gRPC method name.
- Never infer branch URLs from `ClientConn.Target()` or `egrpc.Load()`.

## DTM Server Deployment

DTM runs as a standalone service. EgoAdmin uses the `dtmdriver-ego` plugin for etcd-based service discovery.

```yaml
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

DTM server config example:

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
Production DTM requires its own database. Do not share it with business databases to avoid a single point of failure.
:::

Register the DTM driver in your application:

```go
import (
  _ "github.com/dtm-labs/dtmdriver-ego"
  _ "github.com/dtm-labs/client/dtmgrpc"
)

func init() {
  // dtmdriver-ego auto-registers the etcd driver on startup
  // No additional setup call needed
}
```

## Complete Saga Example: Create Order

Saga is suitable for cross-service flows that can be compensated. This example demonstrates the full orchestration of "create order -> reserve stock -> deduct payment."

**Application layer** (`application/order_usecase.go`):

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
      // forward: reserve stock
      uc.stockTarget+"/stock.v1.StockService/Reserve",
      // compensate: release stock
      uc.stockTarget+"/stock.v1.StockService/Release",
      &stockpb.ReserveRequest{
        OrderId: req.OrderID,
        Items:   toStockItems(req.Items),
      },
    ).
    Add(
      // forward: deduct payment
      uc.payTarget+"/payment.v1.PaymentService/Deduct",
      // compensate: refund
      uc.payTarget+"/payment.v1.PaymentService/Refund",
      &paymentpb.DeductRequest{
        OrderId: req.OrderID,
        UserId:  req.UserID,
        Amount:  req.TotalAmount,
      },
    )

  // Create pending order record locally
  if err := uc.orderRepo.CreatePending(ctx, req); err != nil {
    return err
  }

  // Submit the global transaction
  return saga.Submit()
}
```

**Branch Handler -- Stock Service** (`controller/stock_dtm_grpc.go`):

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

## Saga Example (Basic)

```go
gid := dtmgrpc.MustGenGid(dtmServer)

err := dtmgrpc.NewSagaGrpc(dtmServer, gid).
  Add(
    userTarget+"/user.v1.UserDtmService/Action",
    userTarget+"/user.v1.UserDtmService/Compensate",
    req,
  ).
  Submit()
```

## TCC Pattern

TCC (Try / Confirm / Cancel) is suitable for scenarios requiring resource reservation. Unlike Saga, TCC only reserves resources in the Try phase instead of directly deducting.

**Proto definition**:

```protobuf
service PaymentTccService {
  rpc Try(TryDeductRequest) returns (TryDeductResponse);
  rpc Confirm(TryDeductRequest) returns (TryDeductResponse);
  rpc Cancel(TryDeductRequest) returns (TryDeductResponse);
}
```

**Orchestration layer**:

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

**TCC Handler -- Payment Service**:

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

// Try: freeze the amount (reserve resource)
func (c *PaymentTccController) Try(ctx context.Context, req *pb.TryDeductRequest) (*pb.TryDeductResponse, error) {
  barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
  if err != nil {
    return nil, err
  }

  err = barrier.CallWithDB(c.db, func(tx *sql.Tx) error {
    balance, err := queryBalance(ctx, tx, req.GetUserId())
    if err != nil {
      return err
    }
    if balance < req.GetAmount() {
      return errors.New("insufficient balance")
    }
    return freezeAmount(ctx, tx, req.GetOrderId(), req.GetAmount())
  })
  if err != nil {
    return nil, err
  }
  return &pb.TryDeductResponse{}, nil
}

// Confirm: confirm deduction (frozen amount becomes actual deduction)
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

// Cancel: unfreeze amount (release reserved resource)
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
TCC's Try must be idempotent and support null rollback. When Cancel arrives before Try, it should return success (empty compensation). See the Barrier table section.
:::

## Transactional Message Pattern

Suitable for "complete local transaction first, then reliably send a message" scenarios. DTM guarantees eventual message delivery.

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

  // Local transaction: create order
  err := uc.localTransaction(ctx, func(tx *sql.Tx) error {
    if err := insertOrder(ctx, tx, req); err != nil {
      return err
    }
    // Include DTM barrier operation in the same local transaction
    return msg.Prepare(tx)
  })
  if err != nil {
    return err
  }

  return msg.Submit()
}
```

::: tip
The transactional message's Prepare phase must commit in the same local transaction as the business operation. DTM uses the barrier table to record the prepare state, guaranteeing no messages are lost.
:::

## Barrier Table

RM databases participating in DTM local branches need the `dtm_barrier` table. The barrier solves:

- Idempotency for repeated requests.
- Null compensation.
- Hanging prevention.

**Table structure**:

```sql
CREATE TABLE IF NOT EXISTS dtm_barrier (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  gid        VARCHAR(128) NOT NULL COMMENT 'global transaction ID',
  branch_id  VARCHAR(128) NOT NULL COMMENT 'branch ID',
  op         VARCHAR(45)  NOT NULL COMMENT 'operation type: action/compensate/rollback',
  barrier_id VARCHAR(45)  NOT NULL COMMENT 'barrier ID, prevents hanging',
  reason     VARCHAR(45)  NOT NULL COMMENT 'creation reason',
  create_time TIMESTAMP   DEFAULT CURRENT_TIMESTAMP,
  update_time TIMESTAMP   DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY gid_barrier (gid, branch_id, op, barrier_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

::: warning
Production environments must create this table in all databases participating in DTM branches. Missing the barrier table causes idempotency failures.
:::

Typical handler code using barrier:

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

## Compensation Handler Patterns

Compensation handlers are the key to the Saga pattern. Compensation logic must follow these principles.

**Core requirements**:

| Requirement | Description |
|-------------|-------------|
| Idempotent | Multiple calls with the same parameters produce the same result |
| Null rollback | Compensation succeeds when the forward operation was never executed |
| Best-effort rollback | Compensation must not throw unrecoverable errors |

**Compensation function example**:

```go
// ReleaseStock is the compensation for ReserveStock
func (c *StockDtmController) ReleaseStock(ctx context.Context, req *pb.ReserveRequest) (*pb.ReserveResponse, error) {
  barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
  if err != nil {
    return nil, err
  }

  err = barrier.CallWithDB(c.db, func(tx *sql.Tx) error {
    // Check if reservation record exists
    reserved, err := queryReservedStock(ctx, tx, req.GetOrderId())
    if err != nil {
      return err
    }
    // Null rollback: if no reservation, forward was never executed
    if reserved == nil {
      return nil
    }
    // Restore stock
    for _, item := range reserved.Items {
      if err := addStock(ctx, tx, item.ProductId, item.Quantity); err != nil {
        return err
      }
    }
    // Mark as compensated
    return markCompensated(ctx, tx, req.GetOrderId())
  })
  if err != nil {
    return nil, err
  }
  return &pb.ReserveResponse{}, nil
}
```

**Compensation retry strategy**:

```go
// DTM retries failed compensations by default. Configure retry interval and max attempts via saga options.
saga := dtmgrpc.NewSagaGrpc(uc.dtmServer, gid).
  Add(forward, compensate, data).
  SetOptions(map[string]string{
    "RetryLimit":    "3",
    "TimeoutToFail": "30",
    "BranchTimeout": "10",
  })
```

## Cross-Service gRPC Communication

DTM branches communicate via gRPC. EgoAdmin uses etcd as the service discovery backend.

**Client configuration**:

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

**Branch target resolution rules**:

Branch target addresses are read from config files, not inferred from gRPC client connections:

```go
type OrderUseCase struct {
  dtmServer   string // [component.dtm.server]
  stockTarget string // [component.dtm.branch.stock.target]
  payTarget   string // [component.dtm.branch.payment.target]
}
```

Branch URLs are assembled from the target + full gRPC method path:

```go
// Correct: use branch target to assemble
url := uc.stockTarget + "/stock.v1.StockService/Reserve"

// Wrong: do not infer from ClientConn
// url := stockConn.Target() + "/stock.v1.StockService/Reserve"  // ✗
```

::: warning
Never infer branch URLs from `ClientConn.Target()` or `egrpc.Load()`. Service discovery results may differ from expectations.
:::

## Directory Layout

```text
internal/app/user/
├── controller/
│   └── dtm_grpc.go             # branch gRPC controller
├── application/
│   └── transfer_usecase.go     # global transaction orchestration (initiator)
├── domain/
│   └── balance/
└── adapter/persistence/mysql/
    └── balance_repository.go   # branch local transaction changes
```

## Common Issues and Solutions

### DTM Server Connection Failure

```text
Symptom: Submit() returns connection refused
Diagnosis:
1. Confirm DTM service is running and port is reachable
2. Check [component.dtm.server] configuration
3. Verify DTM registration in etcd: etcdctl get /dtm/ --prefix
4. Check DNS resolution inside containers
```

### Branch Execution Timeout

```text
Symptom: Global transaction times out, branch did not complete
Diagnosis:
1. Check if branch handler is blocking (database locks, slow queries)
2. Increase BranchTimeout configuration
3. Check gRPC timeout settings (readTimeout)
4. Review DTM service logs for retry count
```

### Idempotency Failure

```text
Symptom: Same request produces inconsistent results on repeated execution
Diagnosis:
1. Verify dtm_barrier table exists with correct indexes
2. Check if handler correctly uses barrier.CallWithDB()
3. Ensure business operations complete inside barrier.CallWithDB callback
4. Check barrier table for anomalous records
```

### Null Compensation Failure

```text
Symptom: Cancel/Compensate returns "null compensation failed"
Diagnosis:
1. Check if handler verifies business record existence
2. If record does not exist, return nil (success), not error
3. Check barrier table for forward operation records
```

### Hanging Prevention

```text
Symptom: DTM reports "hanging" error
Explanation: Cancel arrived before Try, barrier table recorded it, then Try arrived and was rejected
Action: This is correct behavior. Check why Try arrived so late, consider optimizing timeout values
```

### DTM Status Queries

```bash
# Query global transaction status
curl http://localhost:36789/api/dtmsvr/query?gid=<global-id>

# View all transactions via DTM management UI
# Default address: http://localhost:36789
```

## Rules

- DTM orchestration belongs in `application`.
- Branch APIs are defined from proto.
- Branch handlers modify only the owning service database.
- RM databases need the DTM barrier table.
- Do not infer branch URLs from gRPC client targets.

## Test Cases

| Scenario | Must Cover |
|----------|------------|
| All branches succeed | Global transaction succeeds |
| First branch succeeds, second fails | First branch compensated |
| Compensation arrives repeatedly | Idempotency |
| Try not reached, Cancel arrives first | Null compensation |
| Cancel arrives then Try arrives | Hanging prevention |
| DTM unavailable | Initiator returns clear error |
| Branch service unavailable | DTM retries or observable failure path |

## Validation

```bash
make dev-up
make run
go test -race ./...
make e2e E2E_TIMEOUT=20m
```
