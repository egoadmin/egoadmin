# Official DTM Reference Pack

This file bundles the important official DTM documentation and example behavior directly into the skill. Do not require temporary local documentation clones when using this skill.

Use upstream docs only to refresh this reference pack. The official source topics covered here are: architecture, protocol, pattern selection, Saga, TCC, two-phase message, barrier, Workflow, XA, final success, transaction options, Go SDK/ORM usage, storage, deployment, operations, upgrade compatibility, and AT/other-mode comparison.

## Architecture And Roles

DTM uses three roles:

- AP: application process that starts and orchestrates a global transaction.
- TM: transaction manager, the DTM server. It stores global transaction state and coordinates branches.
- RM: resource manager, usually a business service that owns local data and branch handlers.

A service can be both AP and RM, for example a nested TCC branch that starts another TCC transaction.

DTM server is shared middleware. It is not one DTM instance per business service. DTM can be deployed as multiple replicas with shared high-availability storage.

## Result Protocol

DTM distinguishes definite business results from transient infrastructure failures.

Branch result mapping:

- Success: HTTP `200` or gRPC `OK`.
- Definite business failure: HTTP `409` or gRPC `Aborted`.
- Ongoing normal processing: HTTP `425` or gRPC `FailedPrecondition`.
- Other errors: transient or unknown, retried with backoff.

Do not map timeouts, unavailable services, database connection errors, or `500` responses to business failure. Those are retryable system failures.

Operations that must not return business failure:

- two-phase message downstream branches.
- Saga compensation.
- TCC Confirm and Cancel.

Those operations must be designed to eventually succeed after transient faults are repaired.

## Pattern Selection

Official DTM pattern guidance:

- Use a local transaction for single-service writes.
- Use two-phase message when a local write must atomically trigger downstream work and no rollback is required.
- Use Saga for rollbackable cross-service workflows.
- Use TCC for short high-consistency workflows with explicit resource reservation.
- Use Workflow when the use case must mix Saga, TCC, XA, HTTP, gRPC, and local operations.
- Use XA only when database-level prepare/commit is acceptable and row lock contention is low.

Consistency tendency from strongest to weakest: XA, TCC, two-phase message, Saga. This is about the window and visibility of intermediate states; all are still final-consistency solutions for cross-service transactions.

## Saga

Saga splits a global transaction into ordered short actions and compensations.

Official behavior:

- DTM executes actions in order by default.
- If all actions succeed, the global transaction succeeds.
- If an action fails with definite business failure, DTM calls compensations in reverse order.
- DTM may also call the compensation for the failed branch. Barrier must make empty compensation safe.
- Compensation must be the semantic reverse of the action and must eventually succeed.

Useful options:

- `WaitResult`: wait for one synchronous processing attempt instead of returning immediately after submit.
- `EnableConcurrent`: execute branches concurrently.
- `AddBranchOrder`: express dependencies for concurrent branches.
- `RetryInterval`: set retry interval; branches returning `ONGOING` use fixed-interval retry.
- `TimeoutToFail`: let a Saga time out and roll back; do not combine with non-compensable operations.

Design rules:

- Put rollbackable operations before non-rollbackable operations.
- Non-rollbackable operations should not return business failure.
- Keep remote requests outside long local transactions when possible.
- If a later branch needs data from an earlier branch, prefer a separate query API or use TCC/Workflow when the dependency is essential.

## TCC

TCC has Try, Confirm, and Cancel.

- Try checks business constraints and reserves required resources.
- Confirm commits reserved resources and does not repeat business checks.
- Cancel releases reserved resources.

Official behavior:

- The application process orchestrates Try calls.
- DTM stores registered Confirm/Cancel branch data.
- If the TCC function returns nil, DTM submits and calls Confirm for registered branches.
- If it returns an error, DTM aborts and calls Cancel for registered branches.
- Confirm and Cancel are retried until success.

Design rules:

- Keep TCC short. Long TCC orchestration can roll back when the application process crashes before it completes.
- Try must be idempotent and handle hanging when Cancel arrives first.
- Cancel must handle empty cancel when Try did not run.
- Confirm and Cancel must be idempotent and eventually successful.
- Nested TCC is supported; the nested service then acts as both AP and RM.

## Two-Phase Message

Two-phase message replaces many local message table and transaction-message uses.

Use it when:

- a local DB transaction and downstream branch trigger must be atomic.
- downstream branches do not require rollback.
- downstream branch execution can be retried until success.

Official `DoAndSubmitDB` behavior:

1. Prepare the DTM message transaction.
2. Run the local DB transaction with a barrier record.
3. Submit the message if the local transaction commits.
4. If the application crashes after local commit but before submit, DTM calls QueryPrepared.

QueryPrepared must distinguish:

- committed: continue and submit downstream branches.
- rollbacked: abort the message.
- in-progress: wait for the local transaction result.
- not-started: mark rollbacked so a late local transaction cannot later commit.

Do not replace QueryPrepared with a time delay or a simple "not found means rollback" check. Use DTM barrier semantics.

`Submit` without `DoAndSubmitDB` is closer to ordinary asynchronous message delivery. Use `WaitResult` when the caller must wait for one downstream processing attempt.

## Barrier

Barrier solves distributed branch disorder:

- duplicate delivery.
- empty compensation/cancel.
- hanging when the forward operation arrives after a reverse operation.

Official algorithm:

1. Open a local transaction.
2. Insert a unique key for current `(gid, branch_id, op)`. If duplicate, commit and return success.
3. For reverse operations such as cancel/compensation, also insert the origin operation key. If that succeeds, the origin did not run; commit and return success.
4. Run the business mutation.
5. Commit on success or roll back on error.

Use official helper APIs such as `BranchBarrier.CallWithDB`, `Call`, Redis, and Mongo helpers. Do not hand-roll "check then update" barrier logic.

Supported barrier storage:

- MySQL, PostgreSQL, and compatible relational databases.
- Redis through Lua.
- Mongo through transactions.

## Workflow

Workflow registers a replayable workflow function in the business service. DTM can call back to resume after crashes.

Workflow can mix:

- Saga-style `OnRollback`.
- TCC-style `OnCommit` and `OnRollback`.
- XA through `DoXa`.
- HTTP calls.
- gRPC calls.
- local operations through `Do`.

Official behavior:

- Initialize Workflow with DTM server target, callback target, and server registration.
- Register a named workflow handler.
- Execute by workflow name and gid.
- When the business process crashes, DTM retries the workflow and SDK returns recorded branch results for completed calls.
- Workflow functions must be idempotent.
- Workflow clients need the Workflow interceptor for calls made under workflow contexts.

Workflow does not fit every case. Prefer Saga, TCC, Msg, or XA when one simpler pattern cleanly models the workflow.

## XA

XA delegates commit/rollback to the database resource manager.

Official behavior:

- Branch local transaction prepares with XA.
- DTM submits or aborts the global transaction.
- DTM calls phase-two commit or rollback.
- Phase-two requests may not contain the original request body; parse request data inside the XA local transaction helper.

Use XA when:

- participating databases support XA.
- operations are short.
- there is low row lock contention.
- automatic database rollback is preferable to custom compensation.

Avoid XA for hot shared rows such as inventory counters. Locks remain across prepare/commit and reduce concurrency.

## Transaction Options

Common official transaction options:

- `WaitResult`: wait for one synchronous execution attempt.
- `TimeoutToFail`: timeout Saga, XA, or TCC. For Msg, it controls prepared-but-unsubmitted query timing.
- `RetryInterval`: retry interval for Msg, Saga, XA, and TCC.
- `BranchHeaders`: custom headers/metadata for branch calls.

DTM default retry uses backoff. Returning `ONGOING` asks DTM to retry at the configured fixed interval.

## Go SDK And ORM Notes

Official Go docs recommend the lightweight client module:

```go
github.com/dtm-labs/client/dtmcli
github.com/dtm-labs/client/dtmgrpc
github.com/dtm-labs/client/workflow
```

The DTM server repository also contains matching client source under:

```go
github.com/dtm-labs/dtm/client/dtmcli
github.com/dtm-labs/dtm/client/dtmgrpc
github.com/dtm-labs/dtm/client/workflow
```

Use the module selected by the target project's `go.mod`. Do not switch modules just because an old example used a different import path.

Barrier and XA APIs use standard `*sql.DB` and `*sql.Tx`. When using GORM or another ORM, obtain or construct the ORM object from the underlying SQL connection/transaction as part of the local transaction boundary.

## Deployment, Storage, And Operations

DTM listens on:

- HTTP: `36789`.
- gRPC: `36790`.

DTM storage options:

- Relational database such as MySQL/PostgreSQL for common production use.
- Redis for higher throughput when the durability tradeoff is acceptable.
- BoltDB for local quick start only.

Production rules:

- Use shared HA storage and multiple DTM replicas.
- If using a relational database store, use the primary database rather than read replicas.
- Keep DTM server timezone aligned with database timezone.
- Keep DTM server and database clocks close; official docs call out small clock skew tolerance.
- Use config files when deployment behavior must be explicit; environment variable mapping follows dotted config names such as `MicroService.EndPoint -> MICRO_SERVICE_END_POINT`.

Operations:

- Monitor unfinished global transactions.
- Alert on repeated confirm/cancel failures. A retry count above a small threshold such as 3 usually needs investigation.
- DTM exposes Prometheus metrics on the HTTP port.
- To force quicker retry for a stuck DB-backed transaction, operations can adjust `next_cron_time` after investigation.

Upgrade note:

- DTM protocol changed around the 1.10 series from body-string results to HTTP status and gRPC status semantics.
- Newer server and SDK versions are compatible with both old and new result protocols.
- Prefer gray upgrade of SDKs before upgrading DTM server.

## Other Modes And AT Comparison

DTM docs compare other consistency patterns:

- Local message table: effective but requires a local message table and polling task per producer.
- Transaction messages: similar idea, but queue-specific and still requires query/commit/rollback handling.
- Best-effort notification: useful for business notifications, not a rollback-capable transaction.
- AT: similar goal to XA but implemented at application/driver layer with before/after images.

DTM does not make AT the default. Official AT comparison calls out dirty rollback, partial SQL support, dirty read concerns, and lower concurrency due to locks. In EgoAdmin, do not introduce AT unless there is an explicit architecture decision and implementation support.

## EGO Driver Notes

DTM supports microservice drivers including EGO through `dtm-driver-ego`.

For EgoAdmin:

- DTM server registers itself through the EGO driver.
- Transaction starters import the EGO driver and call `dtmgrpc.UseDriver("dtm-driver-ego")` when URL parsing needs it.
- `dtmgrpc` dials targets itself. Do not build an `egrpc` client just to infer a target string.
- Use explicit DTM server and branch targets from config.

Older EGO examples may use `github.com/dtm-labs/driver-ego`; prefer `github.com/dtm-labs/dtmdriver-ego` for current DTM versions.
