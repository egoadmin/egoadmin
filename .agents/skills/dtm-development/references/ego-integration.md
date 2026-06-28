# EGO Integration

## DTM Server

DTM server uses `dtm-driver-ego` to register itself and discover branch services.

```yaml
MicroService:
  Driver: dtm-driver-ego
  Target: etcd://127.0.0.1:2379/egoadmin-dtm
  EndPoint: 127.0.0.1:36790
```

`Target` is where DTM registers. `EndPoint` is the address other services can reach. In default local development, business services run on the host, so DTM uses host networking and registers `127.0.0.1:36790`.

If every business service runs inside the same Compose network, a container address such as `dtm:36790` can be used instead. Do not use container-only addresses when transaction starters or branch services run on the host.

## Business Service Config

Only services that start global transactions need DTM client config.

```toml
[component.dtm]
enabled = true
server = "etcd:///egoadmin-dtm"
barrierTableName = "dtm_barrier"
requestTimeout = "3s"

[component.dtm.branch.user]
target = "etcd:///egoadmin-user"
```

Services that only expose branch handlers but never start global transactions do not need `[component.dtm]` enabled by default.

## Driver And Resolver Registration

Use the official lightweight Go client in transaction starters unless the project intentionally depends on the DTM server module:

```go
import "github.com/dtm-labs/client/dtmgrpc"
```

Use the current EGO driver module with the pinned DTM server:

```go
import _ "github.com/dtm-labs/dtmdriver-ego"
```

When DTM branch URL parsing needs the EGO driver:

```go
if err := dtmgrpc.UseDriver("dtm-driver-ego"); err != nil {
	return err
}
```

`dtmgrpc` does its own `grpc.Dial("etcd:///...")`. The business process must register the EGO `etcd` gRPC resolver before that call. In EgoAdmin, initialize DTM clients after existing discovery setup and make any future DTM component depend on `discovery.Ready`.

DTM server source uses `github.com/dtm-labs/dtm/client/...`; official runnable examples usually use `github.com/dtm-labs/client/...`. Treat this skill's bundled examples as the durable project reference. Older EGO examples may show `github.com/dtm-labs/driver-ego`; do not copy that old driver import.

## Branch URLs

Use explicit target config and full gRPC methods:

```go
action := userTarget + "/user.v1.UserDtmService/Action"
compensate := userTarget + "/user.v1.UserDtmService/Compensate"
```

Do not:

- call `ClientConn.Target()` to derive a branch URL.
- create an `egrpc` client only to get a target.
- use gateway HTTP paths for DTM gRPC branches.

`dtmgrpc` dials `grpc.Dial("etcd:///...")` itself. EGO discovery works because the business process has registered the EGO `etcd` resolver.

## Docker Networking

Verify all registered endpoints are reachable from the DTM container and the business services. Avoid:

- `0.0.0.0:<port>` as a registered callback address.
- bridge-network container addresses when DTM or business services need to call host-run services.
- container-only addresses when the transaction starter runs on the host and cannot resolve them.
