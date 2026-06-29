# Frontend Development

The frontend app lives in `web/` and uses Vue 3, TypeScript, Element Plus, Pinia, Vue Router and Vite.

## Structure

```text
web/src/
├── api/
├── components/
├── config/
├── i18n/
├── layouts/
├── router/
├── store/
├── styles/
└── views/
```

## Commands

```bash
cd web
pnpm install
pnpm run dev
pnpm run type-check
pnpm run build
```

## API Calls

Business APIs use gRPC HTTP compatibility paths:

```ts
api.post('/user.v1.UserService/GetUserList', {
  page: 1,
  limit: 20,
})
```

Rules:

- Use `api.post`.
- Do not use RESTful GET/PUT/DELETE for business APIs.
- Use protobuf JSON field names.
- Treat backend `uint64` IDs as strings.

## New Page Workflow

1. Add backend proto and implementation.
2. Add frontend API module.
3. Add page component under `views`.
4. Add router module.
5. Add routeMenu permission nodes.
6. Add i18n labels.
7. Run type check and build.

## ID Handling

```ts
const deptId = ref<string>('0')
const roleIds = ref<string[]>([])
```

Avoid converting backend IDs to JavaScript numbers.

## Validation

```bash
cd web
pnpm run type-check
pnpm run contract:gen
pnpm run build
```
