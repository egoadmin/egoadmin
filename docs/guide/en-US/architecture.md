# Microservice Architecture

## Topology

EgoAdmin consists of three independent services. Each service has its own process, config, database and gRPC contracts.

```text
Browser
  -> HTTP POST /api/*
  -> gateway (9001)
      -> gRPC -> user (9102)
      -> gRPC -> idgen (9202)
```

| Service | HTTP | gRPC | Governor | Responsibility |
|---------|------|------|----------|----------------|
| gateway | 9001 | 9002 | 9003 | external HTTP entry, embedded web, upload, API aggregation |
| user | 9101 | 9102 | 9103 | users, roles, departments, auth, permissions, audit logs |
| idgen | 9201 | 9202 | 9203 | Snowflake IDs, segments, machine leases |

## Service Discovery

Services register to etcd and clients use `etcd:///` targets.

```toml
[etcd]
addrs = ["127.0.0.1:2379"]
connectTimeout = "1s"

[registry]
scheme = "etcd"
prefix = "egoadmin"
serviceTTL = "10s"

[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"
dialTimeout = "3s"
```

## Database Boundaries

| Service | Database | Migration Directory |
|---------|----------|---------------------|
| gateway | `egoadmin_gateway` | `atlas/migrations/gateway` |
| user | `egoadmin_user` | `atlas/migrations/user` |
| idgen | `egoadmin_idgen` | `atlas/migrations/idgen` |

Rules:

- A service directly accesses only its own database.
- Cross-service reads/writes go through `internal/client/*` gRPC wrappers.
- Do not apply one service's migration directory to another service's database.

## Runtime Flow

```text
1. Load configs and env overrides
2. Initialize platform clients and components
3. Run Atlas migrations when enabled
4. Wire server/controller/application/adapter
5. Register generated gRPC services
6. Start HTTP and gRPC servers
7. Register to etcd
8. Mark readiness as ready
```

## Directory Contract

```text
internal/app/<service>/
├── server/
├── controller/
├── application/
├── domain/
├── adapter/
└── schema/
```

## Request Flow

```text
Browser
  -> POST /api/user.v1.UserService/GetUserList
  -> gateway HTTP server
  -> authsession middleware
  -> Casbin permission check
  -> grpc-gateway forwarding
  -> user gRPC controller
  -> application use case
  -> domain / repository
  -> MySQL / Redis
```
