# DTM Concepts For EgoAdmin

## Roles

- AP: application process that starts and submits the global transaction.
- TM: shared DTM server that stores transaction state and coordinates branches.
- RM: resource manager, usually an EgoAdmin business service that owns local data and branch handlers.

## Topology

```text
business service / gateway / future service
  -> dtmgrpc starts a global transaction
  -> shared DTM server
  -> DTM server discovers branch services through dtm-driver-ego + etcd
  -> branch service executes local transaction + barrier
```

## Ownership

- DTM server owns transaction status tables in its own store, for example `egoadmin_dtm`.
- A business service owns its own domain database and branch methods.
- `dtm_barrier` is a local RM table. It belongs in the business database only when that service participates in local DTM branches.
- Gateway may start a transaction only when it is the application process for a use case; it must not directly mutate downstream service databases.

## When Not To Use DTM

- Single service write: use local DB transaction.
- Pure read flow: use ordinary gRPC calls.
- Simple asynchronous side effect with no rollback need: consider ordinary async processing or DTM transactional message only if atomic "local write + downstream trigger" is required.

## Required Properties

- Branch APIs are retryable and idempotent.
- Saga compensation and TCC cancel/confirm must eventually succeed.
- Business failure and system failure must be distinguishable.
- Branch handlers must be safe under duplicate delivery, empty compensation, hanging, and reordered requests.
