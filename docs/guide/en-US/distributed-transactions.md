# Distributed Transactions (DTM)

EgoAdmin integrates DTM for cross-service write consistency. Use local database transactions for single-service writes.

## Pattern Selection

| Scenario | Recommended Pattern |
|----------|---------------------|
| single service, single database | local transaction |
| single service, multiple tables | local transaction |
| cross-service writes with compensation | Saga |
| reserve/confirm/cancel workflow | TCC |
| reliable message after local write | transactional message |

## Saga Example

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

## Config

```toml
[component.dtm]
enabled = true
server = "etcd:///egoadmin-dtm"

[component.dtm.branch.user]
target = "etcd:///egoadmin-user"
```

## Rules

- DTM orchestration belongs in `application`.
- Branch APIs are defined from proto.
- Branch handlers modify only the owning service database.
- RM databases need the DTM barrier table.
- Do not infer branch URLs from gRPC client targets.

## Test Cases

- success path
- compensation path
- idempotency
- empty compensation
- hanging prevention
- DTM unavailable
- branch service unavailable

## Validation

```bash
make dev-up
make run
go test -race ./...
make e2e E2E_TIMEOUT=20m
```
