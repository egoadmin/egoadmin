# DTM Distributed Transactions

Read this before adding or changing DTM, distributed transactions, Saga, TCC, transactional messages, barrier tables, compensation/cancel/confirm handlers, or cross-service write consistency.

## Evidence Paths

- `test/docker-compose.yml`
- `test/compose/dtm.yml`
- `test/compose/dtm/conf.yml`
- `configs/<service>/**`
- `internal/app/<service>/{controller,application,domain,adapter}`
- `internal/client/**`
- `atlas/migrations/<service>`
- `test/e2e/**`
- `.agents/skills/dtm-development/**`
- `.claude/skills/dtm-development/**`
- `opensrc/dtmdriver-clients/ego/**`
- `/tmp/dtm.pub/docs/practice/**` when available

## Architecture

DTM is shared middleware, not a business microservice and not one DTM instance per business service.

```text
business service / gateway / future service
  -> dtmgrpc starts a global transaction
  -> shared DTM server
  -> DTM server discovers branch services through dtm-driver-ego + etcd
  -> branch service executes local transaction + barrier
```

Rules:

- DTM server owns its own transaction state store, for example `egoadmin_dtm`.
- `dtm_barrier` belongs to the local RM business database, not the DTM server database.
- Create `dtm_barrier` only in service databases that actually execute DTM local branch transactions.
- Single-service consistency uses a local database transaction, not DTM.
- Cross-service reads/writes still go through gRPC branch APIs; never directly access another service database.

## Local Docker Compose

Local development includes:

- `mysql-dtm` for DTM transaction state.
- `dtm` using fixed image `yedf/dtm:1.19.0`.
- shared `etcd` for EGO service discovery.

EgoAdmin local development normally runs business services on the host with `make run`/`make run.service`, while DTM runs in Docker. Therefore local DTM uses `network_mode: host` so DTM can call host-registered branch services directly. DTM server config should be explicit. Prefer `test/compose/dtm/conf.yml` over relying only on environment variables when there is any uncertainty.

```yaml
Store:
  Driver: mysql
  Host: 127.0.0.1
  User: egoadmin
  Password: egoadmin
  Port: 3309
  Db: egoadmin_dtm

MicroService:
  Driver: dtm-driver-ego
  Target: etcd://127.0.0.1:2379/egoadmin-dtm
  EndPoint: 127.0.0.1:36790
```

`MicroService.EndPoint` must be reachable by business services and by the DTM process. In the default host-service development mode, use a host-reachable address such as `127.0.0.1:36790`. If every business service also runs inside the Compose network, you may use container DNS such as `dtm:36790`, but do not mix that with host-run services. Avoid registering `0.0.0.0:<port>`.

## Business Service Configuration

Only services that start global DTM transactions should enable a DTM client.

```toml
[component.dtm]
enabled = true
server = "etcd:///egoadmin-dtm"
barrierTableName = "dtm_barrier"
requestTimeout = "3s"

[component.dtm.branch.user]
target = "etcd:///egoadmin-user"
```

Keep two target types separate:

- DTM server target: TM address passed to `dtmgrpc`, for example `etcd:///egoadmin-dtm`.
- Branch service target: RM address used by DTM server callback, for example `etcd:///egoadmin-user`.

Build branch URLs from explicit config and the full gRPC method name:

```go
branch := userTarget + "/user.v1.UserDtmService/DoSomething"
```

Do not use `(*grpc.ClientConn).Target()` or an `egrpc.Load()` business client to infer branch URLs. DTM branch URLs are transaction orchestration config and must be explicit, testable, and environment-overridable.

## EGO Driver And Resolver

Transaction starters and the DTM server must both have EGO/etcd discovery support.

DTM server uses the driver through config:

```yaml
MicroService:
  Driver: dtm-driver-ego
  Target: etcd://etcd:2379/egoadmin-dtm
  EndPoint: dtm:36790
```

Transaction starters that use `github.com/dtm-labs/dtm/client/dtmgrpc` with `etcd:///...` must have the EGO `etcd` gRPC resolver registered before the DTM call. In this project, wire future DTM clients after the existing discovery initialization and make a DTM component depend on `discovery.Ready`.

Use the module names that match the pinned DTM `yedf/dtm:1.19.0` server:

```go
import _ "github.com/dtm-labs/dtmdriver-ego"
```

When DTM branch URL parsing needs the DTM EGO driver, select the driver before building DTM calls:

```go
if err := dtmgrpc.UseDriver("dtm-driver-ego"); err != nil {
	return err
}
```

Older examples may import `github.com/dtm-labs/driver-ego` or `github.com/dtm-labs/client/dtmgrpc`; do not copy those into EgoAdmin unless the DTM client version is intentionally changed.

`dtmgrpc` dials `grpc.Dial("etcd:///...")` itself. EGO service discovery works only after the business process has registered the EGO `etcd` resolver. Do not route DTM branch calls through `internal/client/*` wrappers or EGO `egrpc.Load`.

## Layering

- `application` orchestrates DTM Saga/Msg/TCC.
- `controller` only converts protocol data and calls application.
- `domain` does not import DTM SDKs, protobuf, GORM, or EGO.
- `adapter/persistence/mysql` executes local database work and barrier-protected branch work; it does not start global transactions.
- Branch methods belong to the service that owns the local data and only mutate that service database.

Saga sketch:

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

## Barrier And Migrations

- Add `dtm_barrier` only to participating service migration directories.
- Do not add DTM server tables to business service migrations.
- Use DTM barrier helpers inside branch handlers, with the service-owned DB transaction.
- Design branch handlers as retryable and idempotent. Compensation, cancel, and confirm handlers must eventually succeed.

## Testing

Any complete DTM feature must include integration or e2e coverage for:

- success path.
- business failure and compensation.
- duplicate branch requests.
- empty compensation.
- hanging/suspended branch behavior.
- DTM unavailable.
- branch service unavailable.

For gateway-facing workflows, e2e enters through `test/e2e/gateway`. For internal-only transaction infrastructure, add focused integration tests and still run a local smoke test against DTM/etcd when discovery or Compose changes.

## Validation

- `docker manifest inspect yedf/dtm:1.19.0` or Docker Hub tag API check.
- `make dev-up`
- `curl http://127.0.0.1:36789/api/ping`
- `make dev-down`
- `git diff --check`
- `make e2e E2E_TIMEOUT=20m` when a complete runtime path or gateway-facing workflow changes.
