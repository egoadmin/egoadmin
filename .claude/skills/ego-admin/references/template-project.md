# Template-Derived Project

Read this when deriving a project from CoreAdmin/EgoAdmin, adding a new domain, or deciding whether a pattern is template-generic or project-specific.

## Purpose

EgoAdmin is the source of current template rules. A derived project may add domains, services, pages, components, deployment settings, or business-specific shortcuts, but those choices do not automatically become template rules.

The work is to preserve CoreAdmin's durable development chain while adapting names and business rules to the current target project.

## Decision Method

When extending or comparing projects:

1. Identify the user-facing capability being implemented, not the directory layout.
2. Find the closest template capability in EgoAdmin, such as gateway entry, user, role, organization, log, upload, config, permission, or cron behavior.
3. Classify each observed pattern:
   - Framework invariant: must be preserved, for example proto-generated API, gRPC method identity, permission contract, Wire generation, Atlas migration.
   - Template convention: usually preserved, for example controller/service/store layering, `Options` injection, `copier` mapping, frontend routeMenu binding.
   - Project extension: can vary, for example domain model names, data permission rules, page grouping, import/export flows, statistics, async jobs.
   - Historical deviation: works in the target code but may not be the desired template rule. Record it and avoid making it universal.
4. Implement the requested feature according to the current target project's code and CoreAdmin's durable chain.

## Stable Rules To Preserve

- Proto remains the API source.
- HTTP compatibility remains gRPC method POST path plus `body: "*"`, not RESTful routing.
- Controller implements generated gRPC interfaces and performs request conversion, service call, logging, and response conversion.
- Service owns business orchestration, transactions, locks, permission boundaries, and cross-resource rules.
- Service-owned store packages own GORM models, repo/query methods, scopes, migration model lists, and RPC conversion helpers.
- Wire ProviderSets and generated `wire_gen.go` own dependency wiring.
- `RegisterAPIs` creates backend API records and frontend API manifest from gRPC service/methods.
- `routeMenu` and generated `permission-contract.json` constrain role authorization.
- Frontend pages use project router/menu/store/API/component patterns.
- Third-party middleware should be wrapped as EGO-style components before business services depend on it.

## Allowed Business Variation

Derived projects may change:

- Module path and generated package prefix.
- Domain package names and service names.
- Business models, fields, queries, pages, and menu grouping.
- Data permission logic.
- Import/export/statistics/task workflows.
- Which optional components are enabled and wired.

Derived projects should not casually change:

- API generation flow.
- Permission contract flow.
- Runtime validation source.
- Migration source of truth.
- EGO component lifecycle expectations.
- Frontend API/router/routeMenu synchronization.

## Using Existing Code As Evidence

Use existing project code to answer focused questions:

- How did a similar domain express proto request/response and validation?
- Where did it put transactions, locks, data permission, or long-running work?
- How did it map model fields to RPC fields?
- How did it add routeMenu nodes, API bindings, and button permissions?
- Which parts are obviously project-specific and should not be copied into template rules?

Avoid answers shaped like "create the same file path tree as another project". Prefer answers shaped like "for this capability, preserve these invariants, then adapt names and business rules to the current target".

## New Domain Workflow

1. Define the capability and data ownership.
2. Decide involved layers: proto, controller, service, store, migration, permission, frontend, component, cron/job, or deployment.
3. Read the corresponding references in this skill.
4. Compare target project code and CoreAdmin template examples only for the involved layers.
5. Implement the smallest coherent cross-layer change.
6. Run generation and validation for every touched layer.

## Do Not

- Do not copy project-specific nouns into template rules.
- Do not assume a derived project's omissions are approved simplifications.
- Do not force a template project to match another project's folder shape.
- Do not skip proto, Wire, API manifest, routeMenu, permission contract, or migration just because an existing example appears simple.
- Do not choose a theoretical architecture over current target code without explaining the migration.

## Validation

- Backend generation and tests: `make gen`, then relevant `go test -race` packages.
- Frontend contract: `cd web && pnpm run build`.
- Permission: inspect `api-manifest.ts`, `routeMenu.ts`, and `permission-contract.json`.
- Migration: check model list, migration SQL, and `atlas.sum` when data changes.
- Complete gateway-facing features: run or justify `make e2e E2E_TIMEOUT=20m`.
