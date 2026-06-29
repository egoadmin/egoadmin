# gRPC API Design

The IDGen service exposes its capabilities through two independent gRPC services: `SegmentService` for segment allocation and `MachineLeaseService` for machine lease management. Other business services access IDGen through the `internal/client/idgenclient` client.

## Overview

IDGen's gRPC API is organized into three categories:

| Category | Service Name | Responsibility |
|----------|-------------|----------------|
| Segment allocation | `SegmentService` | Create/allocate ID segments, health check |
| Machine lease | `MachineLeaseService` | Allocate/renew/release process-level machine leases |
| Health check | `SegmentService.Health` | Check segment allocator availability |

Service address ports:

| Service | HTTP | gRPC | Governor |
|---------|------|------|----------|
| idgen | 9201 | 9202 | 9203 |

::: tip Client Usage
Non-idgen services access the IDGen service through `idgenclient.Client`, which is automatically injected by Wire and uses etcd for service discovery.
:::

## Core Usage

### Proto Service Definition

IDGen defines two independent gRPC services for managing segments and machine leases:

```protobuf
// Segment service: provides segment allocation for in-process ID generators
service SegmentService {
    // EnsureSegment creates a segment definition when it does not already exist
    rpc EnsureSegment(EnsureSegmentRequest) returns (EnsureSegmentResponse);
    // AllocateSegment allocates a non-overlapping half-open range [start, end)
    rpc AllocateSegment(AllocateSegmentRequest) returns (AllocateSegmentResponse);
    // Health checks whether the segment allocator is available
    rpc Health(SegmentServiceHealthRequest) returns (SegmentServiceHealthResponse);
}

// Machine lease service: provides process-level machine leases for ID generators
service MachineLeaseService {
    // AllocateLease allocates or renews a machine lease for an instance
    rpc AllocateLease(AllocateLeaseRequest) returns (AllocateLeaseResponse);
    // RenewLease renews an existing machine lease
    rpc RenewLease(RenewLeaseRequest) returns (RenewLeaseResponse);
    // ReleaseLease releases an existing machine lease
    rpc ReleaseLease(ReleaseLeaseRequest) returns (ReleaseLeaseResponse);
}
```

### Interface Details

#### 1. SegmentService

**EnsureSegment** -- Create missing segment definitions:

```protobuf
message EnsureSegmentRequest {
    string namespace   = 1;  // Namespace isolates ID streams between deployments
    string name        = 2;  // Business ID stream name
    int64  next_id     = 3;  // First ID when creating the segment
    int64  step        = 4;  // Base allocation step
    int64  min_step    = 5;  // Minimum step
    int64  max_step    = 6;  // Maximum step
    int32  status      = 7;  // Status: 1 enabled, 2 disabled
    string description = 8;  // Description
}
```

**AllocateSegment** -- Allocate a segment range:

```protobuf
message AllocateSegmentRequest {
    string namespace      = 1;  // Namespace
    string name           = 2;  // Business name
    int64  requested_step = 3;  // Desired step size; server clamps to policy
}

message AllocateSegmentResponse {
    int64 start    = 1;  // Range start
    int64 end      = 2;  // Range end (exclusive)
    int64 step     = 3;  // Base step
    int64 min_step = 4;  // Minimum step
    int64 max_step = 5;  // Maximum step
    int32 status   = 6;  // Status
}
```

#### 2. MachineLeaseService

**AllocateLease** -- Allocate a machine lease:

```protobuf
message AllocateLeaseRequest {
    string namespace              = 1;  // Lease namespace
    string instance_id            = 2;  // Instance identifier
    string stable_instance_id     = 3;  // Stable instance ID (e.g., Pod name)
    int32  max_machine_id         = 4;  // Maximum machine ID
    int64  ttl_seconds            = 5;  // Lease TTL in seconds
    int64  renew_interval_seconds = 6;  // Recommended renewal interval in seconds
}

message AllocateLeaseResponse {
    string namespace              = 1;
    string instance_id            = 2;
    string session_id             = 3;  // Current lease session ID
    int32  machine_id             = 4;  // Allocated machine ID
    int64  ttl_seconds            = 5;
    int64  renew_interval_seconds = 6;
    google.protobuf.Timestamp expires_at = 7;  // Expiry time
}
```

**RenewLease** -- Renew a lease:

```protobuf
message RenewLeaseRequest {
    string namespace              = 1;
    string instance_id            = 2;
    string session_id             = 3;
    int32  machine_id             = 4;
    int64  ttl_seconds            = 5;
    int64  renew_interval_seconds = 6;
}
```

**ReleaseLease** -- Release a lease:

```protobuf
message ReleaseLeaseRequest {
    string namespace   = 1;
    string instance_id = 2;
    string session_id  = 3;
    int32  machine_id  = 4;
}
```

### Client Usage

Non-idgen services access IDGen through `idgenclient.Client`. The client is built via EGO's `egrpc.Load` and automatically uses etcd service discovery:

```go
// internal/client/idgenclient/client.go
type Client struct {
    conn         *egrpc.Component
    Segment      SegmentService       // Segment operations
    MachineLease MachineLeaseService  // Lease operations
}

// Construction (automatic service discovery)
func NewClient(_ discovery.Ready) *Client {
    conn := egrpc.Load("client.grpc.idgen").Build()
    return &Client{
        conn:         conn,
        Segment:      &segmentClient{...},
        MachineLease: &machineLeaseClient{...},
    }
}
```

Client interface definitions:

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

### Wire Injection

In `platform/idgen/provider.go`, the client is automatically assembled into `SegmentStore` and `MachineAllocator`:

```go
var ProviderSet = wire.NewSet(
    NewSegmentStore,       // grpcstore.New(client) -> SegmentStore
    NewMachineAllocator,   // grpcallocator.New(client) -> MachineAllocator
    NewMachineLeaseManager,
    NewComponent,
    NewIDGetter,
)
```

Callers only need to depend on `idgen.Interface` or `idgen.Generator` -- no direct gRPC awareness required.

## Configuration Examples

### Client Configuration

```toml
[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"    # Service discovery address
debug = true                       # Enable debug logging
readTimeout = "3s"                 # Read timeout
dialTimeout = "5s"                 # Dial timeout
```

### Server-side gRPC Configuration

```toml
[server.grpc]
port = 9202                        # gRPC listen port
```

### Server Registration

gRPC services are registered in `server/grpc_server.go`:

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

## Real-World Examples

### Client-side Segment Allocation

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

### Client-side Machine Lease Management

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

### Trace Context Propagation

The client automatically propagates tracing metadata (B3, W3C Trace Context, etc.) by extracting from inbound metadata and injecting into outbound requests via `outgoingContext`:

```go
var forwardedMetadataKeys = map[string]struct{}{
    "authorization":     {},
    "x-request-id":      {},
    "x-correlation-id":  {},
    "x-b3-traceid":      {},
    "x-b3-spanid":       {},
    "traceparent":       {},
    "tracestate":        {},
    // ... more tracing headers
}
```

## How It Works

### gRPC Call Chain

```text
Business service (gateway/user)
    |
    v
idgenclient.Client (egrpc.Load("client.grpc.idgen"))
    |
    v
etcd service discovery (etcd:///egoadmin-idgen)
    |
    v
IDGen service gRPC Server (port 9202)
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
MySQL (idgen_segment / idgen_machine_lease tables)
```

### Error Code Mapping

The gRPC controller maps internal errors to standard gRPC status codes:

| Internal Error | gRPC Status Code | Description |
|----------------|-----------------|-------------|
| `ErrInvalidConfig` | `InvalidArgument` | Invalid configuration parameter |
| `ErrNameNotFound` | `NotFound` | Segment name does not exist |
| `ErrNameDisabled` | `FailedPrecondition` | Segment is disabled |
| `ErrMachineLeaseLost` | `FailedPrecondition` | Machine lease lost |
| `ErrMachineIDOverflow` | `ResourceExhausted` | Maximum machine ID exceeded |
| `ErrStoreUnavailable` | `Unavailable` | Store unavailable |
| `ErrSegmentConflict` | `Unavailable` | Segment update conflict |
| `context.Canceled` | `Canceled` | Request canceled |
| `context.DeadlineExceeded` | `DeadlineExceeded` | Request timed out |
| Other errors | `Internal` | Internal error |

The client's `normalizeError` reverses this mapping, ensuring callers use unified error handling:

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

### Idempotency

- **AllocateLease**: Repeated calls for the same `instanceID` return the existing lease (reuses unexpired instance leases).
- **EnsureSegment**: Uses `ON CONFLICT DO NOTHING` semantics; no side effects when already exists.
- **RenewLease**: Validates `sessionID` and `machineID`; returns `ErrMachineLeaseLost` on mismatch.
- **ReleaseLease**: Validates the tuple `(namespace, machineID, instanceID, sessionID)`; returns error on mismatch.

### Timeout and Retry Strategy

| Operation | Recommended Timeout | Retry Strategy |
|-----------|-------------------|----------------|
| `AllocateSegment` | `fetchTimeout` (2s) | No retry (avoids ID gaps) |
| `AllocateLease` | `renewTimeout` (5s) | Exponential backoff (`reallocateBackoff` 2s) |
| `RenewLease` | `renewTimeout` (5s) | Retry until TTL expiration |
| `Health` | 1s | No retry |

::: warning Do Not Retry Segment Allocation
The range returned by `AllocateSegment` has been consumed. If the client retries but the original request actually succeeded, the range is leaked (creating an ID gap). The segment prefetch mechanism handles retries within the process.
:::

### gRPC Reflection for Debugging

The IDGen server enables gRPC reflection for debugging with `grpcurl`:

```bash
# List all services
grpcurl -plaintext 127.0.0.1:9202 list

# List service methods
grpcurl -plaintext 127.0.0.1:9202 describe idgen.v1.SegmentService

# Health check
grpcurl -plaintext 127.0.0.1:9202 idgen.v1.SegmentService.Health

# Allocate a segment
grpcurl -plaintext -d '{"namespace":"egoadmin-local","name":"user_id","requested_step":100000}' \
    127.0.0.1:9202 idgen.v1.SegmentService.AllocateSegment
```

### Database Table Structure

Segment store table `idgen_segment`:

| Column | Type | Description |
|--------|------|-------------|
| `namespace` | VARCHAR(64) PK | Namespace |
| `name` | VARCHAR(128) PK | Business name |
| `next_id` | BIGINT | Next allocatable ID |
| `step` | BIGINT | Base step |
| `min_step` | BIGINT | Minimum step |
| `max_step` | BIGINT | Maximum step |
| `status` | INT | Status: 1 enabled, 2 disabled |
| `last_step` | BIGINT | Last actual fetch step |
| `last_fetch_at` | DATETIME | Last segment fetch time |

Machine lease table `idgen_machine_lease`:

| Column | Type | Description |
|--------|------|-------------|
| `namespace` | VARCHAR(64) PK | Lease namespace |
| `machine_id` | INT PK | Machine ID |
| `instance_id` | VARCHAR(255) | Instance ID |
| `session_id` | VARCHAR(64) | Lease session ID |
| `ttl_millis` | BIGINT | Lease TTL in milliseconds |
| `renew_millis` | BIGINT | Renewal interval in milliseconds |
| `expires_at` | DATETIME | Expiry time |
| `last_renewed_at` | DATETIME | Last renewal time |

### Expired Lease Cleanup

The IDGen server cleans up expired leases via a cron job (`cron.idgen.machine.cleanup`):

```go
// Default retention 7 days, up to 1000 rows per cleanup
const defaultMachineCleanupRetention = 7 * 24 * time.Hour
const defaultMachineCleanupLimit = 1000
```

## Common Issues

### Connection Failure

**Symptoms**: `ErrStoreUnavailable` or gRPC `Unavailable` error.

**Diagnosis**:

- Is the IDGen service running (port 9202)?
- Is etcd reachable and has the service registered?
- Is `dialTimeout` too short?

```bash
# Check service registration in etcd
etcdctl get /egoadmin/ --prefix | grep idgen

# Test gRPC connection directly
grpcurl -plaintext 127.0.0.1:9202 idgen.v1.SegmentService.Health
```

### Segment Allocation Conflict

**Symptoms**: `ErrSegmentConflict`.

**Cause**: Multiple IDGen instances update the same segment row concurrently. GORM uses `SELECT ... FOR UPDATE` + `WHERE next_id = ?` with optimistic locking; returns conflict when `RowsAffected != 1`.

**Resolution**: The segment allocator retries automatically within the process.

### Lease Renewal Failure

**Symptoms**: `ErrMachineLeaseLost`.

**Diagnosis**:

- Is the Redis or MySQL connection healthy?
- Has the lease been claimed by another process (`sessionID` mismatch)?
- Is the `cron.idgen.machine.renew` cron job running normally?
- Is `renewTimeout` sufficient (default 5 seconds)?

### Client Timeout

**Symptoms**: `context.DeadlineExceeded`.

**Diagnosis**:

- Is `readTimeout` reasonable (recommended 3 seconds)?
- Is the database responding slowly?
- Is segment fetch latency abnormal (check `idgen_segment_fetch_seconds` metric)?

### JavaScript Precision Loss

::: warning
gRPC `int64` fields may lose precision in JSON serialization. Use idcodec to encode as string, or define ID fields as `string` type.
:::

## Reference Links

- `api/proto-internal/idgen/v1/idgen.proto` -- Proto service and message definitions
- `api/gen/go/idgen/v1/idgen_grpc.pb.go` -- Generated gRPC stubs
- `internal/app/idgen/controller/segment_grpc.go` -- Segment gRPC controller
- `internal/app/idgen/controller/machine_grpc.go` -- Machine lease gRPC controller
- `internal/app/idgen/controller/errors.go` -- Error code mapping
- `internal/app/idgen/server/grpc_server.go` -- gRPC server construction and registration
- `internal/client/idgenclient/client.go` -- gRPC client implementation
- `internal/component/idgen/store/grpcstore/store.go` -- Segment gRPC store adapter
- `internal/component/idgen/machine/grpcallocator/allocator.go` -- Machine lease gRPC allocator
