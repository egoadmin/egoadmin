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

### Casbin Model 定义

Casbin 使用 PERM 模型（Policy, Effect, Request, Matchers）描述权限规则。EgoAdmin 采用 RBAC with resource roles 模型：

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _
g2 = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && (r.act == p.act || p.act == "*")
```

说明：

- `r = sub, obj, act`：请求三元组 — 主体（用户角色）、客体（API 资源标识）、操作（方法名）。
- `p = sub, obj, act`：策略定义，与请求对应。
- `g = _, _`：用户-角色继承关系（RBAC）。
- `g2 = _, _`：角色-资源组映射，用于 DataScope 场景。
- `keyMatch2`：支持路径参数匹配，如 `/user.v1.UserService/:method`。
- `e = some(where (p.eft == allow))`：任一 allow 策略即通过。

Gateway 中 Casbin enforcer 初始化：

```go
// internal/app/gateway/adapter/casbin/enforcer.go
func NewEnforcer(adapter persist.Adapter) (*casbin.Enforcer, error) {
  modelText := `...` // 嵌入上述 model 文本
  m, err := model.NewModelFromString(modelText)
  if err != nil {
    return nil, err
  }
  e, err := casbin.NewEnforcer(m, adapter)
  if err != nil {
    return nil, err
  }
  e.AddFunction("keyMatch2", util.KeyMatch2Func)
  return e, nil
}
```

::: tip
Casbin 策略存储在 `egoadmin_user` 数据库的 `casbin_rule` 表中，通过 `gorm-adapter` 读写。
:::

## API Manifest

gateway 启动时根据注册的 gRPC 方法生成 API manifest，前端通过常量绑定权限。

### Proto 生成 Manifest

Proto 文件中每个 RPC 需要 OpenAPI 扩展注解：

```protobuf
// proto/user/v1/user_service.proto
service UserService {
  rpc GetUserList(GetUserListRequest) returns (GetUserListResponse) {
    option (google.api.http) = {
      get: "/user.v1.UserService/GetUserList"
    };
    option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
      security: {
        security_requirement: {
          key: "BearerAuth";
          value: {}
        }
      }
    };
  }

  rpc AddUser(AddUserRequest) returns (AddUserResponse) {
    option (google.api.http) = {
      post: "/user.v1.UserService/AddUser"
      body: "*"
    };
    option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
      security: {
        security_requirement: {
          key: "BearerAuth";
          value: {}
        }
      }
    };
  }
}
```

执行 `make gen` 后，gateway 注册所有 gRPC 方法。构建前端时生成：

```ts
// web/src/api/api-manifest.ts（构建时自动生成，不要手动编辑）
export const APIs = {
  user: {
    v1: {
      UserService: {
        GetUserList: '/user.v1.UserService/GetUserList',
        GetUser: '/user.v1.UserService/GetUser',
        AddUser: '/user.v1.UserService/AddUser',
        UpdateUser: '/user.v1.UserService/UpdateUser',
        DeleteUser: '/user.v1.UserService/DeleteUser',
      },
      RoleService: {
        GetRoleList: '/user.v1.RoleService/GetRoleList',
        AddRole: '/user.v1.RoleService/AddRole',
        UpdateRole: '/user.v1.RoleService/UpdateRole',
        DeleteRole: '/user.v1.RoleService/DeleteRole',
      },
      DeptService: {
        GetDeptTree: '/user.v1.DeptService/GetDeptTree',
        AddDept: '/user.v1.DeptService/AddDept',
        UpdateDept: '/user.v1.DeptService/UpdateDept',
        DeleteDept: '/user.v1.DeptService/DeleteDept',
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

::: warning
`api-manifest.ts` 是构建产物，不要手动编辑。每次 proto 变更后执行 `make gen` 再 `cd web && pnpm run build`。
:::

## routeMenu 菜单权限

`web/src/config/routeMenu.ts` 是菜单、页面、按钮权限声明的源头。

```ts
// web/src/config/routeMenu.ts
import { MenuType } from '@/types/menu'
import { APIs } from '@/api/api-manifest'

// 用户管理模块
{
  id: 30200,
  type: MenuType.Catalog,
  name: 'route.user',
  path: '/user',
  title: '用户管理',
  locale: 'menu.user',
  icon: 'ep:setting',
  children: [
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
        {
          id: 30204,
          type: MenuType.Button,
          title: '删除用户',
          apis: [APIs.user.v1.UserService.DeleteUser],
        },
      ],
    },
    {
      id: 30301,
      parentId: 30200,
      type: MenuType.Page,
      name: 'route.role',
      path: '/user/role',
      title: '角色管理',
      locale: 'menu.user.role',
      icon: 'ep:avatar',
      apis: [APIs.user.v1.RoleService.GetRoleList],
      children: [
        {
          id: 30302,
          type: MenuType.Button,
          title: '新增角色',
          apis: [APIs.user.v1.RoleService.AddRole],
        },
        {
          id: 30303,
          type: MenuType.Button,
          title: '编辑角色',
          apis: [APIs.user.v1.RoleService.UpdateRole],
        },
        {
          id: 30304,
          type: MenuType.Button,
          title: '删除角色',
          apis: [APIs.user.v1.RoleService.DeleteRole],
        },
      ],
    },
    {
      id: 30401,
      parentId: 30200,
      type: MenuType.Page,
      name: 'route.dept',
      path: '/user/dept',
      title: '部门管理',
      locale: 'menu.user.dept',
      icon: 'ep:office-building',
      apis: [APIs.user.v1.DeptService.GetDeptTree],
      children: [
        {
          id: 30402,
          type: MenuType.Button,
          title: '新增部门',
          apis: [APIs.user.v1.DeptService.AddDept],
        },
        {
          id: 30403,
          type: MenuType.Button,
          title: '编辑部门',
          apis: [APIs.user.v1.DeptService.UpdateDept],
        },
      ],
    },
  ],
}
```

### routeMenu 数据结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | number | 菜单/按钮唯一 ID，需全局唯一 |
| `parentId` | number | 父级 ID，顶级为 `0` 或省略 |
| `type` | MenuType | `Catalog`（目录）、`Page`（页面）、`Button`（按钮） |
| `name` | string | 路由名称，用于 `router.push({ name })` |
| `path` | string | 路由路径 |
| `title` | string | 显示标题 |
| `locale` | string | i18n 键名 |
| `icon` | string | Element Plus 图标名 |
| `apis` | string[] | 绑定的 API 权限标识数组 |
| `children` | RouteMenu[] | 子菜单/按钮 |

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

### 合约内容

```json
{
  "version": "1.0.0",
  "generatedAt": "2026-06-29T12:00:00Z",
  "menuMap": {
    "30201": {
      "title": "用户管理",
      "apis": ["/user.v1.UserService/GetUserList"]
    },
    "30202": {
      "title": "新增用户",
      "apis": ["/user.v1.UserService/AddUser"]
    },
    "30203": {
      "title": "编辑用户",
      "apis": ["/user.v1.UserService/UpdateUser"]
    }
  }
}
```

### 合约校验流程

1. 前端构建时遍历 `routeMenu.ts`，将每个菜单/按钮 ID 及其绑定的 API 列表写入 `permission-contract.json`。
2. User 服务新增/编辑角色时读取合约文件。
3. 请求中携带角色要授予的 `menuIDs`。
4. 后端从合约中收集这些 menuID 对应的所有 API 标识，称为 `allowedAPIs`。
5. 请求中同时携带角色要授予的 `apis`。
6. 校验 `apis` 是 `allowedAPIs` 的子集，否则拒绝。
7. 校验通过后写入角色-菜单关联和角色-API 关联。

```go
// internal/app/user/application/usecase/role_usecase.go
func (uc *RoleUseCase) AddRole(ctx context.Context, cmd AddRoleCommand) error {
  // 1. 从合约获取 menuIDs 对应的 allowedAPIs
  allowedAPIs := uc.contract.CollectAPIs(cmd.MenuIDs)

  // 2. 校验 cmd.APIs 是 allowedAPIs 的子集
  if !isSubset(cmd.APIs, allowedAPIs) {
    return errors.New("角色 API 超出菜单允许范围")
  }

  // 3. 写入数据库
  return uc.tx.RunInTx(ctx, func(txCtx context.Context) error {
    role, err := uc.roles.Create(txCtx, cmd.Role)
    if err != nil {
      return err
    }
    if err := uc.permissions.ReplaceRoleMenus(txCtx, role.ID, cmd.MenuIDs); err != nil {
      return err
    }
    return uc.permissions.ReplaceRoleAPIs(txCtx, role.ID, cmd.APIs)
  })
}
```

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

### DataScope 实现

DataScope 结构定义：

```go
// internal/app/user/domain/permission/datascope.go
type DataScope struct {
  Scope     DataScopeType
  UserID    uint64
  DeptID    uint64
  DeptIDs   []uint64   // 自定义部门集合
  SkipCheck bool       // admin/root 绕过
}
```

查询层通用过滤函数：

```go
// internal/app/user/adapter/persistence/mysql/datascope.go
func applyDataScope(db *gorm.DB, scope permission.DataScope, tableAlias ...string) *gorm.DB {
  if scope.SkipCheck {
    return db // admin/root 不加过滤
  }

  alias := ""
  if len(tableAlias) > 0 {
    alias = tableAlias[0] + "."
  }

  switch scope.Scope {
  case permission.DataScopeSelf:
    return db.Where(alias+"owner_user_id = ?", scope.UserID)
  case permission.DataScopeDept:
    return db.Where(alias+"dept_id = ?", scope.DeptID)
  case permission.DataScopeDeptTree:
    deptIDs := getDeptTree(scope.DeptID) // 递归获取子部门 ID
    return db.Where(alias+"dept_id IN ?", deptIDs)
  case permission.DataScopeCustom:
    return db.Where(alias+"dept_id IN ?", scope.DeptIDs)
  case permission.DataScopeAll:
    return db // 不加过滤
  default:
    return db.Where("1 = 0") // 未知范围，拒绝访问
  }
}
```

Repository 使用：

```go
func (r *UserRepository) List(ctx context.Context, q user.Query, scope permission.DataScope) ([]*user.User, int64, error) {
  db := r.db.WithContext(ctx).Model(&UserModel{})

  db = applyUserFilters(db, q)
  db = applyDataScope(db, scope)

  var total int64
  if err := db.Count(&total).Error; err != nil {
    return nil, 0, err
  }

  var rows []UserModel
  if err := db.Offset(int((q.Page - 1) * q.Limit)).Limit(int(q.Limit)).Find(&rows).Error; err != nil {
    return nil, 0, err
  }

  return toUsers(rows), total, nil
}
```

### Admin/Root 绕过机制

```go
// internal/app/user/application/usecase/user_query.go
func (uc *UserUseCase) GetUserList(ctx context.Context, q user.Query) (*user.ListResult, error) {
  auth := permission.FromContext(ctx)

  scope := permission.DataScope{
    Scope:     auth.DataScope,
    UserID:    auth.UserID,
    DeptID:    auth.DeptID,
    DeptIDs:   auth.DeptIDs,
    SkipCheck: auth.IsAdmin || auth.IsRoot,
  }

  return uc.users.List(ctx, q, scope)
}
```

admin/root 用户绕过数据权限；普通用户默认不能越权访问。

::: warning
`SkipCheck` 只应在 auth 层判断，不要在 Repository 中硬编码角色名。
:::

## Role CRUD 与权限分配

### 角色数据模型

```go
// internal/app/user/adapter/persistence/mysql/role_model.go
type RoleModel struct {
  ID          uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  Name        string    `gorm:"column:name;type:varchar(64);not null;uniqueIndex;comment:角色名"`
  Code        string    `gorm:"column:code;type:varchar(64);not null;uniqueIndex;comment:角色编码"`
  DataScope   int32     `gorm:"column:data_scope;type:tinyint;not null;default:1;comment:数据范围"`
  Sort        int32     `gorm:"column:sort;not null;default:0;comment:排序"`
  Status      int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:状态"`
  Remark      string    `gorm:"column:remark;type:varchar(255);default:'';comment:备注"`
  CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}
```

### 关联表

```go
// 角色-菜单关联（权限分配）
type RoleMenuModel struct {
  ID     uint64 `gorm:"primaryKey;autoIncrement"`
  RoleID uint64 `gorm:"column:role_id;not null;index"`
  MenuID uint64 `gorm:"column:menu_id;not null;index"`
}

func (RoleMenuModel) TableName() string { return "role_menu" }

// 用户-角色关联
type UserRoleModel struct {
  ID     uint64 `gorm:"primaryKey;autoIncrement"`
  UserID uint64 `gorm:"column:user_id;not null;index"`
  RoleID uint64 `gorm:"column:role_id;not null;index"`
}

func (UserRoleModel) TableName() string { return "user_role" }
```

### 新增角色并分配权限

```go
// internal/app/user/application/usecase/role_usecase.go
func (uc *RoleUseCase) AddRole(ctx context.Context, cmd AddRoleCommand) (*role.Role, error) {
  allowedAPIs := uc.contract.CollectAPIs(cmd.MenuIDs)
  if !isSubset(cmd.APIs, allowedAPIs) {
    return nil, errors.New("角色 API 超出菜单允许范围")
  }

  var result *role.Role
  err := uc.tx.RunInTx(ctx, func(txCtx context.Context) error {
    r := &role.Role{
      Name:      cmd.Name,
      Code:      cmd.Code,
      DataScope: cmd.DataScope,
      Sort:      cmd.Sort,
      Status:    cmd.Status,
    }
    created, err := uc.roles.Create(txCtx, r)
    if err != nil {
      return err
    }
    result = created

    // 分配菜单权限
    if err := uc.permissions.ReplaceRoleMenus(txCtx, created.ID, cmd.MenuIDs); err != nil {
      return err
    }
    // Casbin 策略写入
    for _, api := range cmd.APIs {
      if err := uc.enforcer.AddPolicy(created.Code, api, "*"); err != nil {
        return err
      }
    }
    return nil
  })
  return result, err
}
```

### 编辑角色权限

```go
func (uc *RoleUseCase) UpdateRole(ctx context.Context, cmd UpdateRoleCommand) error {
  allowedAPIs := uc.contract.CollectAPIs(cmd.MenuIDs)
  if !isSubset(cmd.APIs, allowedAPIs) {
    return errors.New("角色 API 超出菜单允许范围")
  }

  return uc.tx.RunInTx(ctx, func(txCtx context.Context) error {
    if err := uc.roles.Update(txCtx, cmd.Role); err != nil {
      return err
    }

    // 先删除旧 Casbin 策略
    oldAPIs := uc.permissions.GetRoleAPIs(txCtx, cmd.Role.ID)
    for _, api := range oldAPIs {
      uc.enforcer.RemovePolicy(cmd.Role.Code, api, "*")
    }

    // 替换菜单关联
    if err := uc.permissions.ReplaceRoleMenus(txCtx, cmd.Role.ID, cmd.MenuIDs); err != nil {
      return err
    }

    // 写入新 Casbin 策略
    for _, api := range cmd.APIs {
      if _, err := uc.enforcer.AddPolicy(cmd.Role.Code, api, "*"); err != nil {
        return err
      }
    }
    return uc.enforcer.SavePolicy()
  })
}
```

## 常见权限问题与排查

| 问题 | 可能原因 | 解决方案 |
|------|----------|----------|
| 用户能访问未授权页面 | 前端路由未鉴权，或菜单 ID 配置错误 | 检查 `routeMenu` 中 `apis` 是否正确绑定 |
| 普通用户调用 protected API 成功 | Casbin 策略缺失或 enforcer 未正确加载 | 检查 `casbin_rule` 表是否有对应策略，重启 gateway |
| admin 用户被拦截 | admin 标识未正确注入 AuthContext | 检查 `authsession` 中 admin 判断逻辑 |
| 角色分配 API 时报错 | API 不在 permission-contract 范围内 | 先在 `routeMenu` 中添加菜单/按钮，再构建合约 |
| DataScope 不生效 | Repository 未调用 `applyDataScope` | 检查查询方法是否传入并使用 scope |
| Casbin 策略不一致 | 编辑角色时未先删除旧策略 | 编辑角色时先 RemovePolicy 再 AddPolicy |
| 多端登录后 token 失效 | `maxLoginClient` 配置过小 | 调整配置或使用 token 刷新机制 |
| 心跳离线误触发 | `heartbeatOfflineSeconds` 设置过短 | 根据业务场景调大阈值 |

### 调试技巧

1. **查看 Casbin 策略**：

```bash
# 直接查询数据库
mysql -e "SELECT * FROM egoadmin_user.casbin_rule;"
```

2. **临时关闭合约校验**：

```toml
[app.service]
skipPermissionContractCheck = true
```

3. **查看用户权限菜单**：

```bash
# 调用接口获取当前用户菜单
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/user.v1.UserService/GetUserMenus
```

4. **检查路由守卫**：确认 `web/src/router/guard.ts` 中的权限守卫逻辑未被旁路。

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

