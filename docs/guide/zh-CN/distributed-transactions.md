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

## Saga 示例

```go
import (
  "context"

  "github.com/dtm-labs/client/dtmgrpc"
  _ "github.com/dtm-labs/dtmdriver-ego"
)

type TransferUseCase struct {
  dtmServer  string
  userTarget string
}

func (uc *TransferUseCase) Transfer(ctx context.Context, req *TransferRequest) error {
  gid := dtmgrpc.MustGenGid(uc.dtmServer)

  saga := dtmgrpc.NewSagaGrpc(uc.dtmServer, gid).
    Add(
      uc.userTarget+"/user.v1.UserDtmService/FreezeBalance",
      uc.userTarget+"/user.v1.UserDtmService/UnfreezeBalance",
      &FreezeBalanceRequest{
        UserId: req.FromUserID,
        Amount: req.Amount,
      },
    ).
    Add(
      uc.userTarget+"/user.v1.UserDtmService/AddBalance",
      uc.userTarget+"/user.v1.UserDtmService/SubBalance",
      &AddBalanceRequest{
        UserId: req.ToUserID,
        Amount: req.Amount,
      },
    )

  return saga.Submit()
}
```

## Branch API

branch API 从 proto 定义：

```protobuf
service UserDtmService {
  rpc FreezeBalance(FreezeBalanceRequest) returns (FreezeBalanceResponse);
  rpc UnfreezeBalance(UnfreezeBalanceRequest) returns (UnfreezeBalanceResponse);
  rpc AddBalance(AddBalanceRequest) returns (AddBalanceResponse);
  rpc SubBalance(SubBalanceRequest) returns (SubBalanceResponse);
}
```

branch handler 必须位于拥有本地数据的服务内。比如用户余额属于 user 服务，就由 user 服务实现 branch。

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

