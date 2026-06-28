# Barrier

## Purpose

DTM barrier handles:

- idempotency for duplicate branch requests.
- empty compensation when cancel/compensation arrives before action/try.
- hanging when action/try arrives after cancel/compensation.

Use DTM barrier helpers inside local branch handlers instead of handwritten "check then update" logic.

This reference bundles the official DTM barrier behavior so agents do not need temporary local documentation clones.

## Official Algorithm

DTM barrier uses unique-key inserts instead of "check then update":

1. Open a local transaction.
2. Insert the current operation key `(gid, branch_id, op)`. If it already exists, commit and return success.
3. For cancel/compensation/rollback-style operations, insert the origin operation key. If that insert succeeds, the origin operation did not run; commit and return success.
4. Run the business logic.
5. Commit on success or roll back on error.

This handles:

- empty compensation/cancel: the reverse operation arrives before the origin operation.
- hanging: the origin operation arrives after the reverse operation has already completed.
- duplicate delivery: the same branch operation is retried.

## Table Ownership

- `dtm_barrier` belongs to the business service database acting as RM.
- Do not create `dtm_barrier` in every service by default.
- Do not put business service barrier tables in the DTM server database.
- Add the table through the owning service migration directory only when that service has DTM branch handlers.

## Handler Rules

- Wrap local branch business changes with DTM barrier and a local DB transaction.
- Keep branch methods retryable.
- Keep compensation/cancel/confirm idempotent and eventually successful.
- Do not call remote services inside a barrier-protected local transaction unless the design explicitly accepts the lock and retry consequences.
- Keep domain logic independent; barrier usage belongs at application or persistence boundary.
- Use the official `BranchBarrier.CallWithDB`, `Call`, Redis, or Mongo helpers. Do not hand-roll equivalent barrier rows in business code.
- Use standard `*sql.DB` or `*sql.Tx` for DB barrier APIs. If using GORM, convert to the underlying SQL transaction inside the local transaction.

## Saga Branches

Action and compensation should both use barrier protection when they mutate local data. Compensation must be safe when the action did not execute.

## TCC Branches

Try, Confirm, and Cancel should all be idempotent. Cancel must handle empty cancel; Try must handle hanging after Cancel.

## Two-Phase Message QueryPrepared

For message transactions, QueryPrepared uses barrier state to answer whether the local transaction committed, rolled back, is still in progress, or never started. Use the official helper:

```go
bb, err := dtmgrpc.BarrierFromGrpc(ctx)
if err != nil {
	return err
}
err = bb.QueryPrepared(sqlDB)
return dtmgrpc.DtmError2GrpcError(err)
```

Do not replace QueryPrepared with a time-based "not found means rollback after N seconds" rule. Official DTM avoids that race by inserting barrier records with unique keys.

## Storage Support

Official barrier support covers:

- MySQL, PostgreSQL, and compatible relational databases.
- Redis through Lua.
- Mongo through transactions.

For EgoAdmin, prefer MySQL barrier tables in the owning service database unless a branch really owns Redis or Mongo state.

## Migration Checklist

1. Confirm the service really executes DTM local branches.
2. Add `dtm_barrier` to `atlas/migrations/<service>`.
3. Rehash the service migration directory.
4. Apply only to that service database.
5. Add tests for duplicate, empty compensation, and hanging behavior.
