# Transaction Patterns

## Selection

- Local transaction: use for single-service writes.
- DTM transactional message: use when a local write must atomically trigger downstream work and no rollback is required.
- Saga: use for cross-service workflows that can be compensated.
- TCC: use for short high-consistency workflows where visibility and resource reservation matter.
- Workflow: use only when a simpler mode cannot express mixed Saga/TCC/XA/HTTP/gRPC/local operations cleanly.
- XA: do not use as the default in EgoAdmin; consider only with a deliberate architecture decision and low contention.

This matches the official DTM selection guide:

- two-phase message for no-rollback scenarios.
- Saga for rollbackable cross-service workflows.
- TCC for higher consistency and explicit reservation.
- XA for low concurrency cases without row lock contention.

## Saga

Saga splits a global transaction into ordered branch actions and compensations. If a later branch fails, DTM calls compensations in reverse order.

Use Saga when:

- each successful step can be compensated.
- the process can tolerate temporary intermediate states.
- the flow may be long-running.

Rules:

- Compensation must be the semantic reverse of the action.
- Compensation must eventually succeed.
- Barrier-protect action and compensation handlers.
- DTM may call compensation for a branch whose action returned failure; barrier must make empty compensation safe.
- If using `EnableConcurrent`, define dependencies with `AddBranchOrder` when order matters.
- Use `RetryInterval` and `ONGOING` for long external confirmations that should retry at a fixed interval.
- Put non-rollbackable operations after rollbackable operations, and design them so they do not return business failure.
- Use `TimeoutToFail` carefully; do not combine timeout rollback with branches that cannot be compensated.

## TCC

TCC has Try, Confirm, and Cancel.

- Try checks business conditions and reserves resources.
- Confirm commits reserved resources.
- Cancel releases reserved resources.

Use TCC when:

- intermediate states must not be externally visible.
- resources can be reserved explicitly.
- the transaction is short.

Rules:

- Confirm and Cancel must be idempotent and eventually successful.
- Try must handle hanging when Cancel arrives first.
- Keep Try/Confirm/Cancel branch APIs owned by the service that owns the data.
- Keep TCC short. Official DTM TCC stores orchestration in the application process; if the application crashes before completing Try orchestration, DTM can roll back registered branches but cannot invent missing orchestration.
- Nested TCC is supported, but the nested service must understand it is both AP and RM.

## Transactional Message

Use DTM message when a local DB transaction and downstream branch trigger must be atomic and no rollback is needed.

Rules:

- Provide a query-prepared branch when required.
- Use barrier support for the local transaction state.
- Do not model compensating workflows as messages; use Saga or TCC.
- `DoAndSubmitDB` is the normal Go helper for atomic local DB write plus DTM message submit.
- QueryPrepared distinguishes committed, rollbacked, in-progress, and not-started local transactions.
- Message branches are eventually successful; do not return business failure from a branch unless the design accepts permanent message failure.
- `Submit` without `DoAndSubmitDB` can replace ordinary async messages when no local transaction atomicity is needed.
- `WaitResult = true` makes submit wait for one synchronous branch execution attempt.

## Workflow

Workflow lets a business service register a replayable workflow function and lets DTM call back to resume after crashes. It can mix:

- Saga-style `OnRollback`.
- TCC-style `OnCommit` and `OnRollback`.
- XA with `DoXa`.
- HTTP, gRPC, and local `Do` operations.

Rules:

- Use Workflow only when Saga/TCC/Msg/XA alone would be awkward or would duplicate orchestration.
- Initialize Workflow with a DTM server target and a callback endpoint reachable by DTM.
- Add workflow interceptors to clients used inside workflow contexts.
- Make the workflow function idempotent; DTM may replay it and return recorded branch results.
- Preserve official result mapping: gRPC `Aborted` means business failure, `FailedPrecondition` means ongoing.

## XA

XA delegates rollback/commit to the database resource manager.

Use XA when:

- all participating resources support XA.
- branch operations are short.
- row lock contention is low.
- the team accepts lower concurrency for simpler rollback logic.

Rules:

- Do not use XA for shared hot rows such as inventory counters.
- Keep body parsing inside the XA local transaction helper because phase-two callbacks may not include the original request body.
- Prefer Saga/TCC/Msg when service-level semantics and compensation are clearer than database-level prepare/commit.

## AT And Other Modes

DTM docs include AT comparison and other common patterns for design context:

- Local message table and transaction-message patterns are usually replaced by DTM two-phase message.
- Best-effort notification is a business notification pattern, not a full rollback-capable transaction.
- AT is not a default EgoAdmin mode. Official DTM docs compare it with XA and generally recommend XA where XA fits, while noting AT's dirty rollback, SQL support, and dirty read constraints.

## Business Failure vs System Failure

- Business failure: validation such as insufficient balance; DTM should rollback or fail according to the chosen pattern.
- System failure: timeout, unavailable service, transient database error; DTM should retry where the pattern requires eventual success.

Map errors clearly so DTM can make the correct retry/rollback decision.
