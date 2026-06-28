# DTM Testing

## Required Coverage For Complete DTM Features

Cover:

- success path.
- business failure and compensation.
- duplicate branch requests.
- empty compensation.
- hanging/suspended branch behavior.
- DTM unavailable.
- branch service unavailable.

## Test Layers

- Unit tests: branch URL construction, config parsing, error mapping, application orchestration decisions.
- Integration tests: barrier-protected branch handlers against a real database.
- e2e tests: full gateway-facing workflow across real services, DTM, etcd, and databases.

## Official Behavior References

Map new tests to the closest official DTM behavior described in this skill:

- Saga gRPC success, rollback, ongoing, headers, and barrier-protected compensation.
- TCC gRPC success, rollback, nested TCC, Try hanging, empty Cancel, and Confirm/Cancel idempotency.
- Two-phase message success, `DoAndSubmitDB`, QueryPrepared committed/rollbacked/in-progress/not-started behavior, and duplicate delivery.
- Workflow replay, recorded branch results, and mixed Saga/TCC/XA/local operations.
- XA gRPC success, rollback, phase-two callbacks, and low-contention constraints.

Use [go-sdk-examples.md](go-sdk-examples.md), [transaction-patterns.md](transaction-patterns.md), and [barrier.md](barrier.md) as the bundled example and behavior source. Adapt behavior and API shape, but keep EgoAdmin package layering and service ownership.

## Local Smoke

For Compose-only DTM changes:

```bash
docker manifest inspect yedf/dtm:1.19.0
make dev-up
curl http://127.0.0.1:36789/api/ping
make dev-down
git diff --check
```

If Docker Hub is temporarily unavailable, use Docker Hub tag API evidence and state that manifest validation could not run.

## e2e Rule

Gateway-facing workflows enter through `test/e2e/gateway`.

Do not claim a complete cross-service transaction is validated by only testing one branch handler. The proof must include DTM coordination and service discovery unless the change is intentionally limited to local helper code.
