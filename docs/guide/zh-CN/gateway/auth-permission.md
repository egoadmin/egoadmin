# 鉴权与权限控制

Gateway 在将请求转发到下游服务之前，通过认证中间件和 Casbin RBAC 权限校验保护所有受控 API。

## 概述

Gateway 的鉴权体系分为三层：Bearer token 认证、API 分类过滤、Casbin 权限校验。公开接口无需任何认证，登录类接口仅验证 token 有效性，受保护接口在认证通过后还需通过 Casbin 策略匹配才能放行。root 和 admin 用户自动绕过权限检查。

```text
请求进入 Gateway
  -> 提取 Authorization: Bearer <token>
  -> openPack?     放行，无需认证
  -> justLoginPack?  验证 token 后放行，无需 Casbin
  -> 默认:           验证 token + Casbin RBAC 校验
  -> r.sub == "root" || r.sub == "admin"?  自动放行
  -> 转发到下游 gRPC 服务
```

## 核心用法

### API 分类（三档权限包）

Gateway 定义了三档 API 权限分类，通过 gRPC 方法全路径前缀匹配：

```go
// internal/app/gateway/server/grpc_server.go

// openPack: 开放访问，无需登录和鉴权
var openPack = []string{
    "/grpc.health.v1.Health/",             // gRPC 健康检查
    "/user.v1.UserService/Login",          // 登录接口
    "/user.v1.UserService/GetLoginCrypto", // 获取登录加密参数
    "/user.v1.UserService/GetCaptcha",     // 获取验证码
}

// justLoginPack: 仅需登录，无需 Casbin 鉴权
var justLoginPack = []string{
    "/user.v1.UserService/HeartBeatUser", // 心跳上报
    "/user.v1.CenterService",             // 个人中心（前缀匹配，包含所有方法）
    "/user.v1.UserService/Logout",        // 退出登录
    "/user.v1.UserService/GetMenus",      // 获取菜单
}
```

| 分类 | 认证 | Casbin | 典型场景 |
|------|------|--------|----------|
| `openPack` | 否 | 否 | 登录、验证码、健康检查 |
| `justLoginPack` | 是 | 否 | 菜单获取、退出登录、心跳、个人中心 |
| 默认（未在上述列表中） | 是 | 是 | 用户管理、角色管理、部门管理等 |

::: warning 路径匹配规则
权限包使用前缀匹配。如果需要仅放通某个包中的个别方法，必须写全路径。例如 `/user.v1.CenterService` 放通了该服务的所有方法，而 `/user.v1.UserService/Logout` 仅放通 Logout 一个方法。
:::

### Bearer Token 认证

认证流程由 `remoteAuthContext` 函数实现。它从 gRPC metadata 中提取 Bearer token，然后调用 user 服务的 `InternalAuth.ValidateAccessToken` 验证。

```go
// internal/app/gateway/server/grpc_server.go
func remoteAuthContext(ctx context.Context, opts controller.Options) (context.Context, error) {
    // 检查是否在开放包中，是则跳过认证
    if jwtIgnoreFunc(opts)(ctx) {
        return ctx, nil
    }

    // 提取 Bearer token
    rawToken, err := extractBearerToken(ctx)
    if err != nil {
        return nil, err
    }

    // 调用 user 服务验证 token
    auth, err := opts.UserClient.InternalAuth.ValidateAccessToken(ctx, rawToken)
    if err != nil {
        return nil, err
    }

    // 将认证上下文存入 context
    return authsession.NewContext(ctx, auth), nil
}
```

Token 提取逻辑严格校验 `Authorization` 头格式：

```go
func extractBearerTokenFromValue(ctx context.Context, value string) (string, error) {
    if value == "" {
        return "", ecodev1.ErrorUnauthenticated().WithMessage("AuthMissingToken")
    }
    parts := strings.SplitN(value, " ", 2)
    if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
        return "", ecodev1.ErrorUnauthenticated().WithMessage("AuthMissingToken")
    }
    return parts[1], nil
}
```

### Casbin RBAC 权限校验

通过认证后，受保护接口还需经过 Casbin 权限校验。校验由 `permCheckFunc` 实现：

```go
// internal/app/gateway/server/grpc_server.go
func permCheckFunc(opts controller.Options) func(ctx context.Context) (bool, error) {
    return func(ctx context.Context) (bool, error) {
        fullMethod := grpc.FromContext(ctx)

        // 如果在开放或仅登录包中，跳过鉴权
        if lo.ContainsBy(append(devOpen, append(openPack, justLoginPack...)...), func(pack string) bool {
            return strings.HasPrefix(fullMethod, pack)
        }) {
            return true, nil
        }

        // 读取用户信息
        auth, ok := authsession.FromContext(ctx)
        if !ok {
            return false, nil
        }

        // 拆分 service 和 method
        service, method, ok := splitFullMethod(fullMethod)
        if !ok {
            return false, nil
        }

        // 调用 user 服务进行 Casbin 校验
        ok, err := opts.UserClient.InternalAuth.CheckPermission(ctx, auth, service, method)
        if err != nil {
            return false, err
        }

        return ok, nil
    }
}
```

Casbin 校验的实际执行在 user 服务中完成。Gateway 通过 gRPC 调用 `InternalAuth.CheckPermission`。

### Casbin 模型定义

user 服务使用经典的 RBAC 模型，包含角色继承和 root/admin 超级用户机制：

```text
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) == true \
    && r.obj == p.obj \
    && r.act == p.act \
    || r.sub == "root" || r.sub == "admin"
```

| 字段 | 含义 |
|------|------|
| `r.sub` | 请求主体（用户名） |
| `r.obj` | 请求对象（gRPC service 全大写，如 `USER.V1.USERSERVICE`） |
| `r.act` | 请求动作（gRPC method 全大写，如 `ADDUSER`） |
| `g(r.sub, p.sub)` | 角色继承检查：用户是否属于策略中定义的角色 |
| `r.sub == "root" \|\| r.sub == "admin"` | root 和 admin 用户自动放行所有操作 |

::: info root/admin 绕过
Casbin matcher 中的 `r.sub == "root" || r.sub == "admin"` 条件使得 root 和 admin 用户无需匹配任何策略即可访问所有受保护接口。这是设计上的超管机制，不可通过策略配置关闭。
:::

### 权限契约（Permission Contract）

前端权限契约文件 `permission-contract.json` 内嵌在 gateway 二进制中，定义了每个前端菜单可访问的 API 列表。

```json
{
  "system:user": {
    "name": "用户管理",
    "apis": [
      "USER.V1.USERSERVICE/ADDUSER",
      "USER.V1.USERSERVICE/DELETEUSER",
      "USER.V1.USERSERVICE/UPDATEUSER",
      "USER.V1.USERSERVICE/GETUSER",
      "USER.V1.USERSERVICE/GETUSERLIST"
    ]
  },
  "system:role": {
    "name": "角色管理",
    "apis": [
      "USER.V1.ROLESERVICE/ADDROLE",
      "USER.V1.ROLESERVICE/DELETEROLE",
      "USER.V1.ROLESERVICE/UPDATEROLE",
      "USER.V1.ROLESERVICE/GETROLELIST"
    ]
  }
}
```

Gateway 在启动时校验权限契约的完整性。校验逻辑在 `PermissionUseCase.EnsurePermissionContract` 中：

```go
// internal/app/gateway/application/permission_usecase.go
func (uc *PermissionUseCase) EnsurePermissionContract(ctx context.Context) error {
    if uc.skipContractCheck() {
        return nil
    }
    _, err := uc.loadPermissionContract(ctx)
    return err
}
```

权限契约的作用是限制角色可授予的 API 范围。创建或编辑角色时，`ValidateRoleAPIBoundary` 检查请求的 API 是否在菜单契约允许的范围内。

### API 字典同步

Gateway 启动时从 proto 生成的 API Catalog 同步 API 元数据到数据库：

```go
if err := opts.apiSrv.SyncFromCatalog(context.Background(), egoadmin.APICatalog); err != nil {
    return nil, err
}
```

API 字典记录了所有 gRPC 方法的 service path 和 method name，用于权限校验时的 ID 到路径映射。

## 配置示例

### JWT 配置（user 服务）

```toml
# configs/user/config.toml

[app.user]
jwtExpire = 604800              # access token 有效期 7 天（秒）
refreshTokenExpire = 2592000    # refresh token 有效期 30 天（秒）
jwtSignKey = "local-egoadmin-jwt-sign-key"
multiLoginEnabled = true        # 多端登录
maxLoginClient = 2              # 最大同时登录客户端数
```

::: warning 密钥安全
`jwtSignKey` 等敏感配置不能提交生产值。生产环境通过 `EGOADMIN_*` 环境变量覆盖。
:::

### 权限契约校验开关

```toml
[app.service]
skipPermissionContractCheck = false  # 生产环境必须为 false
```

开发调试时可设为 `true` 跳过前端权限契约启动校验。

### Casbin 配置

Casbin 模型通过代码内联传递，不使用外部配置文件。策略数据存储在 `casbin_rule` 表中：

```sql
-- casbin_rule 表结构
CREATE TABLE casbin_rule (
    p_type VARCHAR(100),
    v0     VARCHAR(100),  -- sub (角色名)
    v1     VARCHAR(100),  -- obj (service 路径)
    v2     VARCHAR(100),  -- act (method 名)
    v3     VARCHAR(100),
    v4     VARCHAR(100),
    v5     VARCHAR(100)
);
```

## 实战示例

### 请求完整鉴权流程

以「添加用户」接口为例：

```text
1. 客户端发送请求:
   POST /api/user.v1.UserService/AddUser
   Authorization: Bearer eyJhbGciOi...

2. protoc-gen-go-http handler 直接处理
   全路径: /user.v1.UserService/AddUser

3. AuthSession 中间件:
   - 检查 openPack -> 不在列表中
   - 提取 Bearer token
   - 调用 user.InternalAuth.ValidateAccessToken(token)
   - 验证通过，注入 AuthContext 到 context

4. Perm 中间件:
   - 检查 openPack/justLoginPack -> 不在列表中
   - 读取 AuthContext 获取用户名
   - 拆分: service="user.v1.UserService", method="AddUser"
   - 调用 user.InternalAuth.CheckPermission(auth, service, method)

5. user 服务 Casbin 校验:
   - r.sub = "operator1"
   - r.obj = "USER.V1.USERSERVICE"
   - r.act = "ADDUSER"
   - g("operator1", "admin") -> true? 通过
   - 或匹配 p = ("admin", "USER.V1.USERSERVICE", "ADDUSER")
   - 返回 true

6. 请求放行，转发到 user 服务处理
```

### 测试接口鉴权

```bash
# 1. 登录获取 token
TOKEN=$(curl -s -X POST http://localhost:9001/api/user.v1.UserService/Login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "encrypted_password"}' \
  | jq -r '.accessToken')

# 2. 调用受保护接口
curl -X POST http://localhost:9001/api/user.v1.UserService/GetUserList \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"page": 1, "pageSize": 10}'

# 3. 无 token 调用（应返回 Unauthenticated）
curl -X POST http://localhost:9001/api/user.v1.UserService/GetUserList \
  -H "Content-Type: application/json" \
  -d '{}'

# 4. 调用公开接口（无需 token）
curl -X POST http://localhost:9001/api/user.v1.UserService/GetCaptcha \
  -H "Content-Type: application/json" \
  -d '{}'
```

## 工作原理

### 中间件链执行顺序

HTTP 和 gRPC 通道共享相同的中间件链逻辑：

```text
HTTP 请求:
  Recovery -> AuthSession -> Perm -> Validator -> Ecode -> Handler

gRPC 请求:
  Recovery -> AuthSession -> Perm -> Validator -> Ecode -> Handler

流式 gRPC:
  StreamRecovery -> AuthSessionStream -> PermStream -> EcodeStream -> Handler
```

两个通道的 Controller 实例完全相同，鉴权逻辑也完全相同。区别仅在于 HTTP 请求由 protoc-gen-go-http handler 直接处理。

### 认证上下文传递

认证结果通过 `authsession.NewContext` 存入 Go context，在后续中间件和业务逻辑中可读取：

```go
// 存入 context
ctx = authsession.NewContext(ctx, auth)

// 从 context 读取
auth, ok := authsession.FromContext(ctx)
if !ok {
    return false, nil
}
```

`AuthContext` 包含用户 ID、用户名、角色等信息，供权限校验和业务逻辑使用。

### 权限校验数据流

```text
Gateway                       User 服务
   |                              |
   |-- CheckPermission -------->  |
   |   (auth, service, method)    |
   |                              |-> 从 session 获取用户角色
   |                              |-> 查询 Casbin 策略
   |                              |-> 执行 matcher 校验
   |                              |-> root/admin 自动放行
   |<--- true/false -------------|
```

## 常见问题

### 权限被拒绝 (Permission Denied)

**现象**：已登录用户访问接口返回 `PermissionDenied`。

**排查步骤**：

1. 检查 Casbin 策略表：`SELECT * FROM casbin_rule WHERE v0 = '角色名'`
2. 确认用户的 viewMenus（菜单权限）是否包含该接口对应的 API
3. 验证 API 字典是否已同步：gateway 启动时自动从 proto Catalog 同步
4. 检查 permission-contract.json 是否包含对应菜单的 API 声明
5. 确认角色已分配正确权限且策略已更新

### Token 验证失败

**现象**：返回 `Unauthenticated` 错误。

**排查步骤**：

1. 检查 `Authorization` 头格式：`Bearer <token>`（注意 Bearer 后有空格）
2. 确认 token 未过期（默认 7 天）
3. 检查 user 服务是否正常运行
4. 验证 `jwtSignKey` 配置在 gateway 和 user 服务间一致

### 权限契约校验失败

**现象**：Gateway 启动时报权限契约相关错误。

**排查步骤**：

1. 确认前端构建产物中包含 `web/dist/permission-contract.json`
2. 检查契约文件中的 API 路径格式是否全大写
3. 开发环境可临时设置 `skipPermissionContractCheck = true`

### 接口分类遗漏

**现象**：新增接口需要公开访问但被要求认证。

**解决方案**：在 `grpc_server.go` 的 `openPack` 或 `justLoginPack` 中添加对应路径前缀。注意路径匹配使用前缀方式。

## 参考链接

- [Casbin 官方文档](https://casbin.org/docs/overview)
- [Casbin RBAC 模型](https://casbin.org/docs/rbac)
- 项目内相关源码：
  - `internal/app/gateway/server/grpc_server.go` — 认证和权限中间件、API 分类定义
  - `internal/app/gateway/application/permission_usecase.go` — 权限契约和 API 边界校验
  - `internal/app/gateway/domain/permission/policy.go` — 权限策略模型
  - `internal/app/gateway/domain/api/api.go` — API 字典模型
  - `internal/component/authsession/` — 认证会话组件
