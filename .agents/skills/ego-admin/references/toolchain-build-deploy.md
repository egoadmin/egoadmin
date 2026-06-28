# Toolchain, Build, And Deploy

Read this before changing Make targets, Buf/Wire/OpenAPI generation, frontend contract generation, Atlas deployment, Dockerfile, Compose files, CI, release packaging, generated artifacts, or tools.

## Evidence Paths

- `Makefile`
- `buf.gen.yaml`, `buf.gen.internal.yaml`, `buf.work.yaml`
- `atlas/atlas.hcl`, `atlas/apply.sh`, `atlas/migrations/**`
- `atlas/schema/**`
- `cmd/gateway/Dockerfile`
- `cmd/gateway`, `cmd/user`
- `tools/atlasloader`, `tools/egoadminctl`
- `test/docker-compose.yml`, `test/compose/**`
- `test/compose/dtm.yml`, `test/compose/dtm/conf.yml`
- `.woodpecker/**`
- `web/package.json`, `web/scripts/**`, `web/dist/permission-contract.json`

## Generation Chain

`make gen` is canonical. It runs proto generation, internal proto generation, Go generation, and Wire.

Rules:

- Change proto first, then generate.
- Do not patch generated code.
- Do not patch `wire_gen.go`.
- Inspect generated tags/docs when proto validation/copy comments change.

## Build Targets

Important targets:

- `make gen`
- `make build`
- `make build SERVICE=<service>`
- `make service.check SERVICE=<service>`
- `make migrate.new SERVICE=<service> NAME=<change_name>`
- `make migrate.schema SERVICE=<service>`
- `make migrate.hash SERVICE=<service>`
- `make migrate.apply SERVICE=<service> ATLAS_URL=<database-url>`
- `make e2e`

Default packaged gateway behavior currently centers on `cmd/gateway`. If another service needs an image, define its build context and migration behavior explicitly.

## Frontend Contract

`web/dist/permission-contract.json` is generated from frontend source and API manifest. Do not hand-edit it.

Run frontend build after route/menu/API permission changes.

## Docker And Compose

Rules:

- Keep Dockerfile and Makefile in sync.
- Include Atlas binary and migrations where container startup applies migrations.
- `make install` installs the non-community Atlas CLI because GORM `external_schema` requires it. Do not add `--community` to the Atlas install command.
- Keep Compose env vars aligned with config keys.
- Avoid real secrets in Dockerfile, Makefile, Compose, or docs.
- Local development middleware should run via local Docker Compose, split by service where ownership differs.
- Pin middleware image versions. DTM is pinned to `yedf/dtm:1.19.0`; do not use `latest` or floating tags.
- Configure DTM with an explicit `conf.yml` when environment variable support is uncertain.
- Keep DTM as shared middleware with its own `mysql-dtm`; do not deploy one DTM server per business service.
- Ensure DTM `MicroService.EndPoint` is reachable from the DTM container and branch services.

## CI

Do not add `make e2e` to default CI unless the runner clearly provides Docker CLI, Docker Compose, Docker socket or DinD, Atlas CLI, and enough runtime resources. Prefer an explicit e2e stage with those prerequisites documented.

## Generated Artifact Policy

Generated artifacts may be committed only when repository convention requires it and source is regenerated consistently.

Source of truth:

- proto source: `api/proto/**`, `api/proto-internal/**`.
- Wire source: provider sets and constructors.
- frontend source: `web/src/**`.
- permission contract: generated from routeMenu/API manifest.
- migrations: generated/rehashed through Atlas.
- Atlas HCL schema snapshots under `atlas/schema/<dialect>/<service>.hcl`: generated review artifacts from `make migrate.schema`, not hand-maintained source of truth.

## Validation

- `make gen`
- `buf lint`
- `make migrate.schema SERVICE=<service>` when changing migration tooling or requested audit snapshots.
- `cd web && pnpm run build` for frontend/permission changes.
- `make build` or service build target for build path changes.
- `make e2e E2E_TIMEOUT=20m` for local middleware, migration, service startup, or gateway-facing behavior.
- `curl http://127.0.0.1:36789/api/ping` after DTM Compose changes.
