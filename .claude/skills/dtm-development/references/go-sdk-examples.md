# Go SDK Examples

These examples use the official lightweight Go client package shape. Match the exact version in the target project's `go.mod`.

## Imports

```go
import (
	"context"
	"database/sql"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/dtm-labs/client/dtmgrpc"
	"github.com/dtm-labs/client/workflow"
	_ "github.com/dtm-labs/dtmdriver-ego"
	"github.com/go-resty/resty/v2"
	"google.golang.org/protobuf/types/known/emptypb"
)
```

The DTM server repository contains matching client source under `github.com/dtm-labs/dtm/client/...`, while official lightweight clients use `github.com/dtm-labs/client/...`. Treat this file as the durable project reference unless the project intentionally changes the dependency.

## gRPC Result Mapping

Use official helpers so DTM can distinguish success, business failure, ongoing, and transient errors:

```go
func (s *branchServer) Action(ctx context.Context, req *ActionRequest) (*emptypb.Empty, error) {
	if businessFailed {
		return &emptypb.Empty{}, dtmgrpc.DtmError2GrpcError(dtmcli.ErrFailure)
	}
	if stillProcessing {
		return &emptypb.Empty{}, dtmgrpc.DtmError2GrpcError(dtmcli.ErrOngoing)
	}
	return &emptypb.Empty{}, nil
}
```

Do not return `Aborted` for infrastructure failures. Use ordinary errors for transient failures so DTM retries with backoff.

## Saga gRPC

Use Saga for cross-service workflows that can be compensated.

```go
gid := dtmgrpc.MustGenGid(dtmServer)

err := dtmgrpc.NewSagaGrpc(dtmServer, gid).
	Add(userTarget+"/user.v1.UserDtmService/Action", userTarget+"/user.v1.UserDtmService/Compensate", req).
	Add(orderTarget+"/order.v1.OrderDtmService/Action", orderTarget+"/order.v1.OrderDtmService/Compensate", req).
	Submit()
```

Useful options from official docs:

```go
saga := dtmgrpc.NewSagaGrpc(dtmServer, gid)
saga.WaitResult = true
saga.RetryInterval = 60
saga.TimeoutToFail = 1800
saga.EnableConcurrent().AddBranchOrder(2, []int{0, 1})
```

Saga compensation runs in reverse order. DTM may call compensation for a failed branch; barrier must handle empty compensation.

## Saga HTTP

Official HTTP Saga uses the same action/compensation idea with URLs:

```go
req := map[string]any{"amount": 30}

err := dtmcli.NewSaga(dtmHTTPServer, gid).
	Add(busiURL+"/TransOut", busiURL+"/TransOutCompensate", req).
	Add(busiURL+"/TransIn", busiURL+"/TransInCompensate", req).
	Submit()
```

## TCC gRPC

Use TCC for short high-consistency transactions with explicit reservation.

```go
gid := dtmgrpc.MustGenGid(dtmServer)

err := dtmgrpc.TccGlobalTransaction(dtmServer, gid, func(tcc *dtmgrpc.TccGrpc) error {
	reply := &emptypb.Empty{}
	if err := tcc.CallBranch(req,
		userTarget+"/user.v1.UserTccService/Try",
		userTarget+"/user.v1.UserTccService/Confirm",
		userTarget+"/user.v1.UserTccService/Cancel",
		reply,
	); err != nil {
		return err
	}
	return tcc.CallBranch(req,
		orderTarget+"/order.v1.OrderTccService/Try",
		orderTarget+"/order.v1.OrderTccService/Confirm",
		orderTarget+"/order.v1.OrderTccService/Cancel",
		&emptypb.Empty{},
	)
})
```

Try checks constraints and reserves resources. Confirm and Cancel must be idempotent and eventually successful.

## TCC HTTP

Official HTTP TCC registers Confirm/Cancel and immediately calls Try:

```go
err := dtmcli.TccGlobalTransaction(dtmHTTPServer, gid, func(tcc *dtmcli.Tcc) (*resty.Response, error) {
	resp, err := tcc.CallBranch(req, busiURL+"/Try", busiURL+"/Confirm", busiURL+"/Cancel")
	if err != nil {
		return resp, err
	}
	return tcc.CallBranch(req, otherURL+"/Try", otherURL+"/Confirm", otherURL+"/Cancel")
})
```

Nested TCC branch handlers can reconstruct the transaction from context:

```go
tcc, err := dtmgrpc.TccFromGrpc(ctx)
if err != nil {
	return &emptypb.Empty{}, err
}
err = tcc.CallBranch(req, nestedTry, nestedConfirm, nestedCancel, &emptypb.Empty{})
```

## Two-Phase Message gRPC

Use two-phase message when a local transaction must atomically trigger downstream work and there is no rollback requirement.

```go
gid := dtmgrpc.MustGenGid(dtmServer)

msg := dtmgrpc.NewMsgGrpc(dtmServer, gid).
	Add(userTarget+"/user.v1.UserMsgService/DownstreamAction", req)

err := msg.DoAndSubmitDB(
	userTarget+"/user.v1.UserMsgService/QueryPrepared",
	sqlDB,
	func(tx *sql.Tx) error {
		return localWrite(tx, req)
	},
)
```

The QueryPrepared branch is part of the protocol:

```go
func (s *msgServer) QueryPrepared(ctx context.Context, req *MsgRequest) (*emptypb.Empty, error) {
	bb, err := dtmgrpc.BarrierFromGrpc(ctx)
	if err != nil {
		return &emptypb.Empty{}, err
	}
	err = bb.QueryPrepared(sqlDB)
	return &emptypb.Empty{}, dtmgrpc.DtmError2GrpcError(err)
}
```

`Submit` without `DoAndSubmitDB` is closer to ordinary async message delivery. Use `WaitResult = true` when the caller must wait for one synchronous branch execution attempt.

## Two-Phase Message HTTP

Official HTTP message examples use `DoAndSubmitDB` for local DB atomicity:

```go
msg := dtmcli.NewMsg(dtmHTTPServer, gid).
	Add(busiURL+"/TransIn", req)

err := msg.DoAndSubmitDB(busiURL+"/QueryPrepared", sqlDB, func(tx *sql.Tx) error {
	return localWrite(tx, req)
})
```

## Barrier-Protected Branch Handler

Use barrier inside each local mutating branch:

```go
func (s *branchServer) Action(ctx context.Context, req *ActionRequest) (*emptypb.Empty, error) {
	bb, err := dtmgrpc.BarrierFromGrpc(ctx)
	if err != nil {
		return &emptypb.Empty{}, err
	}
	err = bb.CallWithDB(sqlDB, func(tx *sql.Tx) error {
		return repo.ApplyActionTx(tx, req)
	})
	return &emptypb.Empty{}, dtmgrpc.DtmError2GrpcError(err)
}
```

For GORM, obtain the underlying `*sql.Tx` inside a transaction as shown in the official SDK ORM docs. Barrier APIs operate on standard `*sql.DB` and `*sql.Tx`.

## Workflow

Workflow can mix Saga, TCC, XA, HTTP, gRPC, and local operations. Use it only when the simpler modes cannot express the workflow cleanly.

```go
workflow.InitGrpc(dtmServer, callbackTarget, grpcServer)

err := workflow.Register("order-workflow", func(wf *workflow.Workflow, data []byte) error {
	req := decode(data)

	wf.NewBranch().OnRollback(func(bb *dtmcli.BranchBarrier) error {
		return compensate(req)
	})
	if _, err := userClient.Action(wf.Context, req); err != nil {
		return err
	}

	wf.NewBranch().OnCommit(func(bb *dtmcli.BranchBarrier) error {
		return confirm(req)
	}).OnRollback(func(bb *dtmcli.BranchBarrier) error {
		return cancel(req)
	})
	_, err := orderClient.Try(wf.Context, req)
	return err
})

_, err = workflow.ExecuteCtx(ctx, "order-workflow", gid, payload)
```

Workflow requires the workflow callback endpoint to be reachable by DTM. Workflow handlers must be idempotent because DTM can replay them after crashes.

## XA gRPC

Use XA only for deliberate low-contention cases.

```go
gid := dtmgrpc.MustGenGid(dtmServer)

err := dtmgrpc.XaGlobalTransaction(dtmServer, gid, func(xa *dtmgrpc.XaGrpc) error {
	if err := xa.CallBranch(req, userTarget+"/user.v1.UserXaService/Action", &emptypb.Empty{}); err != nil {
		return err
	}
	return xa.CallBranch(req, orderTarget+"/order.v1.OrderXaService/Action", &emptypb.Empty{})
})
```

Local branch handler:

```go
func (s *xaServer) Action(ctx context.Context, req *XaRequest) (*emptypb.Empty, error) {
	err := dtmgrpc.XaLocalTransaction(ctx, dbConf, func(db *sql.DB, xa *dtmgrpc.XaGrpc) error {
		return localWrite(db, req)
	})
	return &emptypb.Empty{}, err
}
```

XA locks database resources across prepare/commit. Avoid high-contention rows such as shared inventory counters.
