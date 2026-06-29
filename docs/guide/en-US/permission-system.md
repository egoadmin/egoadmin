# Permission System

EgoAdmin uses a closed backend/frontend permission chain. Hiding frontend buttons is not considered authorization by itself.

## Chain

```text
authsession Bearer token
  -> API classification
  -> Casbin service/method permission
  -> DataScope query filtering
  -> API manifest
  -> routeMenu.ts
  -> permission-contract.json
```

## API Classification

| Type | Login Required | Casbin Required | Example |
|------|----------------|-----------------|---------|
| public | no | no | login, captcha, login crypto challenge |
| login-only | yes | no | menus, logout, heartbeat, personal center |
| protected | yes | yes | user, role and department management |

## Permission ID

Protected APIs use gRPC identity:

```text
USER.V1.USERSERVICE/ADDUSER
USER.V1.ROLESERVICE/UPDATEROLE
```

Do not use HTTP paths, frontend route paths or menu IDs as backend permission IDs.

## routeMenu

```ts
{
  id: 30201,
  type: MenuType.Page,
  path: '/user/user',
  title: 'User Management',
  apis: [APIs.user.v1.UserService.GetUserList],
  children: [
    {
      id: 30202,
      type: MenuType.Button,
      title: 'Add User',
      apis: [APIs.user.v1.UserService.AddUser],
    },
  ],
}
```

Use permissions in Vue:

```ts
const userStore = useUserStore()
const canAdd = computed(() => userStore.VA(['30202']))
```

## Permission Contract

```bash
cd web
pnpm run build
```

This generates `web/dist/permission-contract.json`. The user service uses it to validate role API grants.

## DataScope

DataScope restricts ordinary users' visible rows. Admin/root users bypass data permission checks.

| Scope | Meaning |
|-------|---------|
| self | own records only |
| department | current department |
| department tree | current department and children |
| all | no filter |
| custom | selected departments |

## Validation

```bash
make gen
cd web && pnpm run build
go test -race ./internal/app/user/...
make e2e E2E_TIMEOUT=20m
```
