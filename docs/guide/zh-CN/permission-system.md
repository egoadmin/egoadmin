# 权限系统

EgoAdmin 的权限设计是后端和前端闭环控制，不依赖单独的隐藏按钮或单独的后端拦截。

## 权限链路

```text
请求进入 gateway
  -> authsession Bearer token 校验
  -> API 分类：public / login-only / protected
  -> protected API 进入 Casbin 校验
  -> application 层执行角色边界和业务权限规则
  -> repository 查询时应用 DataScope 数据权限
  -> 前端 routeMenu 控制菜单和按钮
  -> permission-contract.json 限制角色可授予 API
```

## API 分类

| 类型 | 是否登录 | 是否 Casbin | 示例 |
|------|----------|-------------|------|
| public | 否 | 否 | 登录、验证码、登录加密挑战 |
| login-only | 是 | 否 | 获取菜单、退出登录、心跳、个人中心 |
| protected | 是 | 是 | 用户、角色、部门等管理接口 |

OpenAPI、后端拦截器和前端权限描述必须保持一致。

## Bearer 认证

认证由 `internal/component/authsession` 管理，包含：

- access token 和 refresh token。
- JWT 签名与过期时间。
- 多端登录控制。
- 心跳离线检测。
- 强制下线。
- logout / refresh。

核心配置在 `configs/user/config.toml`：

```toml
[app.user]
adminPassword = "123456"
jwtExpire = 604800
refreshTokenExpire = 2592000
jwtSignKey = "local-egoadmin-jwt-sign-key"
useCaptcha = false
multiLoginEnabled = true
maxLoginClient = 2
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660
revokeSessionOnHeartbeatOffline = false
```

::: warning
`jwtSignKey`、管理员密码、MinIO secret、CDN sign secret 等配置不能提交生产值。生产环境通过环境变量或部署配置覆盖。
:::

## 登录加密

密码传输使用 `LoginCrypto` 挑战机制，不发送明文密码，也不使用 MD5。

前端流程：

```text
1. 输入用户名
2. 调用 GetLoginCrypto(username, ua, action)
3. 后端返回 RSA 公钥、challengeId、nonce、keyId
4. 前端用 Web Crypto RSA-OAEP/SHA-256 加密密码 payload
5. 调用 Login，提交 passwordCipher、keyId、challengeId
6. 后端验证 challenge 和时间戳，解密并校验密码
```

配置：

```toml
[component.logincrypto]
challengeTTL = "3m0s"
timestampSkew = "2m0s"
rsaKeyBits = 4096
enableMetrics = true
```

## Casbin API 权限

protected API 的权限标识使用 gRPC service/method 身份：

```text
USER.V1.USERSERVICE/ADDUSER
USER.V1.ROLESERVICE/UPDATEROLE
USER.V1.DEPTSERVICE/DELETEDEPT
```

规则：

- 不使用 HTTP path 作为权限 ID。
- 不使用前端路由 path 作为权限 ID。
- 不使用菜单 ID 作为后端 API 权限 ID。
- gRPC service/method 改名时，需要同步权限文档、routeMenu、e2e。

## API Manifest

gateway 启动时根据注册的 gRPC 方法生成 API manifest，前端通过常量绑定权限：

```ts
// web/src/api/api-manifest.ts
export const APIs = {
  user: {
    v1: {
      UserService: {
        GetUserList: '/user.v1.UserService/GetUserList',
        AddUser: '/user.v1.UserService/AddUser',
      },
    },
  },
} as const
```

routeMenu 使用常量：

```ts
apis: [
  APIs.user.v1.UserService.GetUserList,
  APIs.user.v1.UserService.AddUser,
]
```

## routeMenu 菜单权限

`web/src/config/routeMenu.ts` 是菜单、页面、按钮权限声明的源头。

```ts
{
  id: 30201,
  parentId: 30200,
  type: MenuType.Page,
  name: 'route.user',
  path: '/user/user',
  title: '用户管理',
  locale: 'menu.user.user',
  icon: 'ep:user',
  apis: [APIs.user.v1.UserService.GetUserList],
  children: [
    {
      id: 30202,
      type: MenuType.Button,
      title: '新增用户',
      apis: [APIs.user.v1.UserService.AddUser],
    },
    {
      id: 30203,
      type: MenuType.Button,
      title: '编辑用户',
      apis: [APIs.user.v1.UserService.UpdateUser],
    },
  ],
}
```

页面中使用：

```ts
const userStore = useUserStore()
const canAdd = computed(() => userStore.VA(['30202']))
```

```vue
<el-button v-if="canAdd" type="primary" @click="openAddDialog">
  新增
</el-button>
```

::: tip
按钮隐藏只是用户体验。真正的访问控制必须由后端 authsession + Casbin 执行。
:::

## permission-contract

构建前端时生成权限合约：

```bash
cd web
pnpm run build
```

输出：

```text
web/dist/permission-contract.json
```

user 服务角色新增/编辑时会读取该合约，校验角色只能被授予已选菜单/按钮允许的 API。

本地调试可以临时跳过：

```toml
[app.service]
skipPermissionContractCheck = true
```

生产环境不应跳过。

## DataScope 数据权限

DataScope 控制普通用户能看到哪些数据。

| 范围 | 含义 | 查询行为 |
|------|------|----------|
| 本人 | 只看自己创建/拥有的数据 | `owner_user_id = current_user` |
| 本部门 | 只看当前部门数据 | `dept_id = current_dept` |
| 本部门及子部门 | 当前部门树 | `dept_id IN (...)` |
| 全部 | 不加数据范围过滤 | admin/root 或授权角色 |
| 自定义 | 指定部门集合 | `dept_id IN (selected_depts)` |

查询层常见用法：

```go
func (r *UserRepository) List(ctx context.Context, q user.Query, scope permission.DataScope) ([]*user.User, int64, error) {
  db := r.db.WithContext(ctx).Model(&UserModel{})

  db = applyUserFilters(db, q)
  db = applyDataScope(db, scope)

  // count + paginate + find
}
```

admin/root 用户绕过数据权限；普通用户默认不能越权访问。

## 新增受保护 API 的检查清单

1. proto RPC 定义完整 OpenAPI auth 描述。
2. RPC 不在 openPack / justLoginPack 时默认为 protected。
3. 执行 `make gen`。
4. server 注册 generated gRPC service。
5. gateway 能生成 api-manifest。
6. `routeMenu.ts` 使用 `APIs` 常量绑定页面/按钮。
7. `cd web && pnpm run build` 生成 `permission-contract.json`。
8. 普通用户越权访问被拒绝。
9. admin 或具备权限角色访问成功。
10. e2e 覆盖成功和拒绝路径。

## 验证命令

```bash
make gen
cd web && pnpm run build
go test -race ./internal/app/user/...
make e2e E2E_TIMEOUT=20m
```

