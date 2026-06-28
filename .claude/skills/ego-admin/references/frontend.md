# Frontend

Read this before changing EgoAdmin `web/` pages, API modules, router, menu, store, routeMenu, permissions, components, styles, runtime config, form validation, or frontend build behavior. For upload components, embedded `web/dist`, SPA fallback, or runtime frontend config injection, also read [upload-static-web.md](upload-static-web.md).

## Project Layout

- `web/` is the same-repository EgoAdmin frontend. It is not a Git submodule.
- `web/dist` is generated output embedded by Go and contains `permission-contract.json`.
- Treat `web/src/**`, `web/scripts/**`, `web/package.json`, and `web/pnpm-lock.yaml` as normal source files owned by this repository.
- Do not edit `web/dist` as source and do not commit `web/node_modules`, `web/dist`, or local frontend cache directories.

## Evidence Paths

- `web/package.json`
- `web/vite.config.ts`
- `web/src/api/index.ts`
- `web/src/api/modules/*.ts`
- `web/src/api/api-manifest.ts`
- `web/src/config/routeMenu.ts`
- `web/src/router/`
- `web/src/store/modules/`
- `web/src/views/`
- `web/src/components/`
- `web/scripts/generate-contract.cjs`
- `web/scripts/permission-guard-sdk.cjs`

## API Modules

Use `src/api/index.ts` and `src/api/modules/<domain>.ts`. Do not scatter raw axios calls.

Current API calls use:

```ts
api.post('/user.v1.RoleService/GetRoleList', data)
```

This is required because the backend exposes gRPC method HTTP POST compatibility entries, not RESTful resource endpoints.

Rules:

- Use `api.post` for business APIs.
- Match proto path `/<package>.<Service>/<Method>`.
- Put request fields in body, aligned to protobuf JSON names.
- Do not change query/update/delete calls into HTTP GET/PUT/DELETE.
- Use generated `APIs` constants from `api-manifest.ts` in `routeMenu`, not hardcoded strings.

API module workflow:

1. Add or extend `src/api/modules/<domain>.ts`.
2. Use `api.post('/<package>.<Service>/<Method>', data)` for every business method.
3. Send protobuf JSON field names in camelCase, for example `deptId`, `roleIds`, `userStatus`.
4. Keep list request shapes aligned with proto request messages: `page`, `limit`, `sort`, `order`, and filters.
5. Do not invent frontend-only request fields unless controller/service explicitly supports them.
6. After backend startup regenerates `api-manifest.ts`, use `APIs.<package>.v1.<Service>.<Method>` in `routeMenu`.

Prefer typed request/response objects when adding new code. Existing modules use `any` in places, but new code should reduce ambiguity when practical.

## Login Crypto Frontend

Password entry flows must use Web Crypto RSA-OAEP/SHA-256 through the project login crypto helper.

Rules:

- Call `GetLoginCrypto` with `username`, `ua`, and explicit `action` before submitting password flows.
- Encrypt an in-memory JSON payload containing `username`, relevant password fields, `challengeId`, `nonce`, `timestamp`, `ua`, and `action`.
- Send only `passwordCipher`, `keyId`, and `challengeId` plus non-secret fields in API requests.
- Use the same challenge flow for login, password change, and password confirmation flows such as changing phone number.
- Refresh-token login sends only the refresh token and does not call `GetLoginCrypto`.
- Keep action constants centralized, for example `login`, `center.edit_password`, and `center.edit_info`.
- Do not store raw password, old password, new password, ciphertext, challenge ID, nonce, token, refresh token, or Authorization headers in logs.
- Do not add MD5 packages such as `spark-md5` or `js-md5`, and do not use frontend hashing as the password transport.

Raw password values may exist briefly in component form state before encryption. They must not be persisted to stores, localStorage, route query, logs, or outgoing request payloads.

## Route, Menu, Store

Routes and permissions must stay aligned:

- `router/modules/*` defines actual routes.
- `routes.ts` registers async route modules.
- `routeMenu.ts` defines permission/menu IDs, paths, titles, and API lists.
- `permission.ts` filters routes from backend menu permissions.
- `route.ts` registers/clears dynamic routes.
- `menu.ts` builds navigation.
- `user.ts` owns token, user info, menus, login/logout, permissions, heartbeat, and `VA(ids)`.

New page workflow:

1. Add `views/<domain>/<page>/index.vue`.
2. Add/extend router module with unique `name`, correct `path`, and `meta`.
3. Add route module to `asyncRoutes` when needed.
4. Add `routeMenu` nodes for page init and action buttons.
5. Gate page and buttons with `userStore.VA(['id'])` or existing auth components.
6. Add API module calls and contract binding.
7. Run frontend validation/build.

Route/menu rules:

- Route `path` and routeMenu `path` must describe the same navigable page.
- Route `name` must be unique and stable.
- `routeMenu` page init node usually contains all APIs needed to load the page.
- Action child nodes should contain only APIs needed by that button/dialog, such as add, edit, delete, detail, reset password, or force logout.
- Use stable numeric IDs. Do not renumber existing IDs because stored role menu strings depend on them.
- For a hidden helper API required by a visible action, bind it to the same action node or the page init node intentionally.
- If a page has no user-facing permission boundary, explain why before leaving it out of routeMenu.

Permission usage:

```ts
const userStore = useUserStore()
const canAdd = computed(() => userStore.VA(['30202']))
```

Use existing page patterns for `v-if`, disabled states, no-permission states, and destructive confirmation dialogs. Do not rely only on disabled buttons; backend permission must also be bound through routeMenu/contract/Casbin.

## Page And Component Patterns

Typical admin pages use:

- `<script setup lang="ts">`.
- `PageMain` for page frame.
- `SearchBar` for filters.
- Element Plus tables/forms/dialogs.
- `Pagination` in table append slot.
- `DefaultPage` for no permission/no data/no search result states.
- `HintDialog` or existing business dialogs for destructive actions.
- `SvgIcon`, `AlIcon`, or Element Plus icons.
- Page-local `listParam`, `loading`, `noData`, `noSearchResults`, and `noPermission` state.

Use existing components first. Extract a new component only when several pages share the same non-trivial structure.

List page state should usually include:

- request params object with `page`, `limit`, `sort`, `order`, and filters.
- `loading` state around API calls.
- `tableData` initialized as an empty array.
- `total` from response count/total.
- `noPermission`, `noData`, and `noSearchResults` states when the existing page pattern supports them.
- reset-search behavior that returns pagination to page 1.
- delete/edit/add success paths that refresh the list or current tree branch.

Dialog/form components should usually:

- receive visible/model props from parent.
- clone input into local form state before editing.
- define Element Plus rules consistent with proto `validate` tags and labels.
- call the API module through parent/service methods, not raw axios.
- emit success/close events and let the page refresh data.

## Field And Validation Sync

- Frontend request fields are camelCase protobuf JSON names.
- Backend protobuf `uint64` and `fixed64` ID fields are JSON strings in Vue/TypeScript.
- List filters and pagination must match request message fields and validation tags.
- Form validation should match proto request/entity `validate` constraints.
- Response display fields must match proto response/entity fields and controller conversions.
- `Dept.childs` maps to organization tree children props.
- Timestamp fields such as `createdAt` and `updatedAt` should use existing date formatting helpers.

When adding a model field shown in the UI:

1. Add DB field and migration when persisted.
2. Add proto request/response/entity field and comments.
3. Add copier tags or mysql conversion methods.
4. Add controller/service mapping.
5. Add frontend request field, response display, form field, and validation rule.
6. Add list search/sort behavior only if mysql supports the filter/sort safely.

Do not add a frontend form field before the proto request and controller actually accept it.

## Protobuf uint64 IDs In Frontend

Backend protobuf `uint64` and `fixed64` fields are serialized to JSON as strings. Treat all backend IDs as strings in Vue/TypeScript.

Rules:

- Do not use `Number(...)`, `parseInt(...)`, unary `+`, numeric `ref(0)`, or `number[]` for backend IDs such as `id`, `userId`, `deptId`, `roleIds`, `workspaceId`, `parentId`, and `ownerUserId`.
- Use `string`, `string[]`, `''`, and `'0'` for ID state and request payloads.
- Element Plus `el-select`, `el-cascader`, and `el-tree` values for backend IDs must preserve string values.
- Compare IDs with string normalization: `` `${a}` === `${b}` ``.
- Only use `number` for true small numeric fields: `page`, `limit`, `status`, `priority`, `maxUses`, `expiresSeconds`, enum values, counters, and pagination.
- Before sending a request containing IDs, inspect the payload shape: large IDs must be quoted JSON strings, not numeric literals.

Why:

JavaScript `number` cannot safely represent 64-bit Snowflake IDs. Converting `236548862796365825` to `number` can silently produce `236548862796365820`, causing backend lookups and ownership checks to fail.

Before finishing frontend code that reads or submits backend IDs:

1. Search changed files for `Number(`, `parseInt(`, `+id`, `number[]`, and `ref(0)`.
2. If the value is a protobuf `uint64` ID, replace it with string handling.
3. Verify cascader/tree/select option values preserve `item.id` as string.
4. Run `pnpm run type-check`.

## Style Rules

- Use existing Element Plus style and project components.
- Keep page-private styles in `<style scoped lang="scss">`.
- Use `:deep(...)` for Element Plus deep selectors.
- Keep global theme changes in `src/styles`.
- Do not change layouts to solve a single page issue.
- Do not introduce another UI library.

## Do Not

- Do not edit `dist` as source.
- Do not add standalone SPA pages outside the existing layout.
- Do not store local form state in global stores.
- Do not add route without routeMenu, or routeMenu without route.
- Do not change permission IDs casually.
- Do not ignore EgoAdmin `web/` contract generation when copying frontend examples.
- Do not use HTTP GET/PUT/DELETE for business APIs.
- Do not hardcode permission API strings when `APIs` constants exist.
- Do not hide a button without adding backend permission coverage.
- Do not send plaintext passwords, MD5 hashes, fixed password hashes, or OPAQUE/stable password-equivalent values to backend APIs.

## Validation

- `cd web && pnpm run type-check`.
- `cd web && pnpm run contract:gen`.
- `cd web && pnpm run build`.
- Manually test non-admin visibility for menus, pages, buttons, and empty states when permissions change.
- Password transport regression search: `rg -n "crypto/md5|md5\\(|spark-md5|js-md5|OPAQUE|passwordauth|passwordCipher|challengeId|nonce" web/src`.
