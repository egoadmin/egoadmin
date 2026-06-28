---
name: dtm-development
description: DTM distributed transaction development guidance with official DTM docs, Go SDK examples, and EgoAdmin integration rules. Use when implementing or reviewing DTM, Saga, TCC, two-phase messages, Workflow, XA, barrier tables, compensation/cancel/confirm handlers, dtm-driver-ego service discovery, branch APIs, or cross-service write consistency in EgoAdmin or derived Go microservices.
---

# DTM Development

## Workflow

1. Decide whether DTM is required. Use a local database transaction for single-service writes.
2. Read the bundled official DTM reference before designing behavior: [official-docs.md](references/official-docs.md).
3. Choose the transaction pattern: read [transaction-patterns.md](references/transaction-patterns.md).
4. For Go implementation shape, read [go-sdk-examples.md](references/go-sdk-examples.md).
5. Identify AP, TM, and RM roles: read [concepts.md](references/concepts.md).
6. Design gRPC branch APIs from proto and service ownership.
7. Configure DTM server and branch targets explicitly: read [ego-integration.md](references/ego-integration.md).
8. Add barrier tables only to participating RM service databases: read [barrier.md](references/barrier.md).
9. Use local Docker Compose DTM for development: read [local-dev-compose.md](references/local-dev-compose.md).
10. Add tests for success, compensation, idempotency, empty compensation, hanging, DTM unavailable, and branch service unavailable: read [testing.md](references/testing.md).

## Official Material Rules

- Treat official DTM docs as the source of truth for transaction semantics.
- Use this skill's references as the bundled local summary of important official docs and examples.
- Do not depend on temporary local clones or scratch paths. If upstream docs are rechecked, update this skill's reference files with the durable content.
- Do not reduce DTM to EgoAdmin-specific wiring. EgoAdmin rules adapt official DTM behavior to this repository; they do not replace official DTM semantics.
- Do not copy imports blindly from old examples. Match the project-selected DTM client module and version.

## Project Rules

- Keep DTM as shared middleware, not an EgoAdmin business microservice.
- DTM server owns its own transaction state store such as `egoadmin_dtm`.
- Business services own their data and branch handlers; branch handlers modify only the owning service database.
- Do not create `dtm_barrier` in every service. Add it only to databases that execute DTM local branches.
- Do not use DTM for a single-service local transaction.
- Do not call another service database directly. Cross-service branches are gRPC methods.
- Keep DTM orchestration in `internal/app/<service>/application`.
- Keep `controller` as protocol conversion only.
- Keep `domain` independent from DTM SDKs, protobuf, GORM, and EGO.
- Keep persistence adapters focused on local data changes and barrier-protected branch work.

## Addressing Rules

Keep DTM server target and branch service target separate:

```toml
[component.dtm]
enabled = true
server = "etcd:///egoadmin-dtm"

[component.dtm.branch.user]
target = "etcd:///egoadmin-user"
```

Build branch URLs from explicit config and full gRPC method names:

```go
branch := userTarget + "/user.v1.UserDtmService/DoSomething"
```

Do not use `(*grpc.ClientConn).Target()` or `egrpc.Load()` to infer branch URLs. `dtmgrpc` dials `etcd:///...` itself; EGO discovery works only after the business process has registered the EGO `etcd` resolver.

## Minimal Saga Shape

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

Prefer the official lightweight Go client package unless the project intentionally uses the DTM server module as a library:

```go
import "github.com/dtm-labs/client/dtmgrpc"
```

The transaction starter must run after EGO discovery initialization and import the EGO driver:

```go
import _ "github.com/dtm-labs/dtmdriver-ego"
```

When DTM branch URL parsing needs the EGO driver:

```go
if err := dtmgrpc.UseDriver("dtm-driver-ego"); err != nil {
	return err
}
```
