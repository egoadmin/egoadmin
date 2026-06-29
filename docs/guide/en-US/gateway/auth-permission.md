# Authentication and Permission Control

The gateway enforces authentication and Casbin RBAC permission checks before forwarding requests to downstream services.

## Overview

The gateway's auth system has three layers: Bearer token authentication, API classification filtering, and Casbin permission validation. Public APIs require no authentication. Login-only APIs only validate token validity. Protected APIs additionally require Casbin policy matching after authentication succeeds. The root and admin users automatically bypass permission checks.

```text
Request enters Gateway
  -> Extract Authorization: Bearer <token>
  -> openPack?     Pass, no authentication needed
  -> justLoginPack?  Validate token then pass, no Casbin
  -> Default:        Validate token + Casbin RBAC check
  -> r.sub == "root" || r.sub == "admin"?  Auto-pass
  -> Forward to downstream gRPC service
```

## Core Usage

### API Classification (Three-Tier Permission Packs)

The gateway defines three API permission tiers, matched by gRPC method full-path prefix:

```go
// internal/app/gateway/server/grpc_server.go

// openPack: Open access, no login or auth needed
var openPack = []string{
    "/grpc.health.v1.Health/",             // gRPC health check
    "/user.v1.UserService/Login",          // Login endpoint
    "/user.v1.UserService/GetLoginCrypto", // Get login crypto params
    "/user.v1.UserService/GetCaptcha",     // Get captcha
}

// justLoginPack: Login required, no Casbin auth needed
var justLoginPack = []string{
    "/user.v1.UserService/HeartBeatUser", // Heartbeat report
    "/user.v1.CenterService",             // Personal center (prefix match, all methods)
    "/user.v1.UserService/Logout",        // Logout
    "/user.v1.UserService/GetMenus",      // Get menus
}
```

| Category | Auth | Casbin | Typical Use |
|----------|------|--------|-------------|
| `openPack` | No | No | Login, captcha, health check |
| `justLoginPack` | Yes | No | Menu retrieval, logout, heartbeat, profile |
| Default (not in above lists) | Yes | Yes | User management, role management, dept management, etc. |

::: warning Path Matching Rules
Permission packs use prefix matching. If you need to open only specific methods within a package, you must write the full path. For example, `/user.v1.CenterService` opens all methods of that service, while `/user.v1.UserService/Logout` opens only the Logout method.
:::

### Bearer Token Authentication

The authentication flow is implemented by the `remoteAuthContext` function. It extracts the Bearer token from gRPC metadata, then calls the user service's `InternalAuth.ValidateAccessToken` to validate it.

```go
// internal/app/gateway/server/grpc_server.go
func remoteAuthContext(ctx context.Context, opts controller.Options) (context.Context, error) {
    // Check if in open pack, skip authentication if so
    if jwtIgnoreFunc(opts)(ctx) {
        return ctx, nil
    }

    // Extract Bearer token
    rawToken, err := extractBearerToken(ctx)
    if err != nil {
        return nil, err
    }

    // Call user service to validate token
    auth, err := opts.UserClient.InternalAuth.ValidateAccessToken(ctx, rawToken)
    if err != nil {
        return nil, err
    }

    // Store auth context in context
    return authsession.NewContext(ctx, auth), nil
}
```

Token extraction strictly validates the `Authorization` header format:

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

### Casbin RBAC Permission Check

After passing authentication, protected APIs still need Casbin permission validation. The check is implemented by `permCheckFunc`:

```go
// internal/app/gateway/server/grpc_server.go
func permCheckFunc(opts controller.Options) func(ctx context.Context) (bool, error) {
    return func(ctx context.Context) (bool, error) {
        fullMethod := grpc.FromContext(ctx)

        // Skip auth if in open or login-only packs
        if lo.ContainsBy(append(devOpen, append(openPack, justLoginPack...)...), func(pack string) bool {
            return strings.HasPrefix(fullMethod, pack)
        }) {
            return true, nil
        }

        // Read user info
        auth, ok := authsession.FromContext(ctx)
        if !ok {
            return false, nil
        }

        // Split service and method
        service, method, ok := splitFullMethod(fullMethod)
        if !ok {
            return false, nil
        }

        // Call user service for Casbin check
        ok, err := opts.UserClient.InternalAuth.CheckPermission(ctx, auth, service, method)
        if err != nil {
            return false, err
        }

        return ok, nil
    }
}
```

The actual Casbin validation executes in the user service. The gateway calls `InternalAuth.CheckPermission` via gRPC.

### Casbin Model Definition

The user service uses a classic RBAC model with role inheritance and a root/admin superuser mechanism:

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

| Field | Meaning |
|-------|---------|
| `r.sub` | Request subject (username) |
| `r.obj` | Request object (gRPC service in uppercase, e.g., `USER.V1.USERSERVICE`) |
| `r.act` | Request action (gRPC method in uppercase, e.g., `ADDUSER`) |
| `g(r.sub, p.sub)` | Role inheritance check: whether user belongs to the role defined in the policy |
| `r.sub == "root" \|\| r.sub == "admin"` | Root and admin users auto-pass all operations |

::: info Root/Admin Bypass
The `r.sub == "root" || r.sub == "admin"` condition in the Casbin matcher allows root and admin users to access all protected APIs without matching any policy. This is an intentional superuser mechanism and cannot be disabled through policy configuration.
:::

### Permission Contract

The frontend permission contract file `permission-contract.json` is embedded in the gateway binary, defining the API list accessible from each frontend menu.

```json
{
  "system:user": {
    "name": "User Management",
    "apis": [
      "USER.V1.USERSERVICE/ADDUSER",
      "USER.V1.USERSERVICE/DELETEUSER",
      "USER.V1.USERSERVICE/UPDATEUSER",
      "USER.V1.USERSERVICE/GETUSER",
      "USER.V1.USERSERVICE/GETUSERLIST"
    ]
  },
  "system:role": {
    "name": "Role Management",
    "apis": [
      "USER.V1.ROLESERVICE/ADDROLE",
      "USER.V1.ROLESERVICE/DELETEROLE",
      "USER.V1.ROLESERVICE/UPDATEROLE",
      "USER.V1.ROLESERVICE/GETROLELIST"
    ]
  }
}
```

The gateway validates the permission contract at startup. The validation logic is in `PermissionUseCase.EnsurePermissionContract`:

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

The permission contract limits the range of APIs grantable to roles. When creating or editing roles, `ValidateRoleAPIBoundary` checks whether the requested APIs fall within the menu contract's allowed scope.

### API Dictionary Sync

The gateway syncs API metadata from the proto-generated API Catalog to the database at startup:

```go
if err := opts.apiSrv.SyncFromCatalog(context.Background(), egoadmin.APICatalog); err != nil {
    return nil, err
}
```

The API dictionary records the service path and method name of all gRPC methods, used for ID-to-path mapping during permission checks.

## Configuration Examples

### JWT Configuration (User Service)

```toml
# configs/user/config.toml

[app.user]
jwtExpire = 604800              # Access token validity 7 days (seconds)
refreshTokenExpire = 2592000    # Refresh token validity 30 days (seconds)
jwtSignKey = "local-egoadmin-jwt-sign-key"
multiLoginEnabled = true        # Multi-device login
maxLoginClient = 2              # Max concurrent login clients
```

::: warning Key Security
Sensitive configs like `jwtSignKey` must not have production values committed. Override production values via `EGOADMIN_*` environment variables.
:::

### Permission Contract Check Toggle

```toml
[app.service]
skipPermissionContractCheck = false  # Must be false in production
```

Set to `true` during development to skip frontend permission contract startup validation.

### Casbin Configuration

The Casbin model is passed inline via code, not through an external config file. Policy data is stored in the `casbin_rule` table:

```sql
-- casbin_rule table structure
CREATE TABLE casbin_rule (
    p_type VARCHAR(100),
    v0     VARCHAR(100),  -- sub (role name)
    v1     VARCHAR(100),  -- obj (service path)
    v2     VARCHAR(100),  -- act (method name)
    v3     VARCHAR(100),
    v4     VARCHAR(100),
    v5     VARCHAR(100)
);
```

## Real-World Examples

### Full Authentication Flow for a Request

Using the "Add User" API as an example:

```text
1. Client sends request:
   POST /api/user.v1.UserService/AddUser
   Authorization: Bearer eyJhbGciOi...

2. protoc-gen-go-http converts to Controller call
   Full path: /user.v1.UserService/AddUser

3. AuthSession middleware:
   - Check openPack -> not in list
   - Extract Bearer token
   - Call user.InternalAuth.ValidateAccessToken(token)
   - Validation passes, inject AuthContext into context

4. Perm middleware:
   - Check openPack/justLoginPack -> not in list
   - Read AuthContext to get username
   - Split: service="user.v1.UserService", method="AddUser"
   - Call user.InternalAuth.CheckPermission(auth, service, method)

5. User service Casbin check:
   - r.sub = "operator1"
   - r.obj = "USER.V1.USERSERVICE"
   - r.act = "ADDUSER"
   - g("operator1", "admin") -> true? Pass
   - Or match p = ("admin", "USER.V1.USERSERVICE", "ADDUSER")
   - Returns true

6. Request allowed, forwarded to user service
```

### Testing API Authentication

```bash
# 1. Login to get token
TOKEN=$(curl -s -X POST http://localhost:9001/api/user.v1.UserService/Login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "encrypted_password"}' \
  | jq -r '.accessToken')

# 2. Call protected API
curl -X POST http://localhost:9001/api/user.v1.UserService/GetUserList \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"page": 1, "pageSize": 10}'

# 3. Call without token (should return Unauthenticated)
curl -X POST http://localhost:9001/api/user.v1.UserService/GetUserList \
  -H "Content-Type: application/json" \
  -d '{}'

# 4. Call public API (no token needed)
curl -X POST http://localhost:9001/api/user.v1.UserService/GetCaptcha \
  -H "Content-Type: application/json" \
  -d '{}'
```

## How It Works

### Middleware Chain Execution Order

HTTP and gRPC channels share the same middleware chain logic:

```text
HTTP request:
  Recovery -> AuthSession -> Perm -> Validator -> Ecode -> Handler

gRPC request:
  Recovery -> AuthSession -> Perm -> Validator -> Ecode -> Handler

Streaming gRPC:
  StreamRecovery -> AuthSessionStream -> PermStream -> EcodeStream -> Handler
```

The Controller instances are identical for both channels, as is the authentication logic. The only difference is that HTTP requests first pass through protoc-gen-go-http conversion.

### Auth Context Propagation

Authentication results are stored in the Go context via `authsession.NewContext` and can be read by subsequent middleware and business logic:

```go
// Store in context
ctx = authsession.NewContext(ctx, auth)

// Read from context
auth, ok := authsession.FromContext(ctx)
if !ok {
    return false, nil
}
```

`AuthContext` contains user ID, username, roles, and other information for permission checks and business logic.

### Permission Check Data Flow

```text
Gateway                       User Service
   |                              |
   |-- CheckPermission -------->  |
   |   (auth, service, method)    |
   |                              |-> Get user role from session
   |                              |-> Query Casbin policies
   |                              |-> Execute matcher validation
   |                              |-> root/admin auto-pass
   |<--- true/false -------------|
```

## Common Issues

### Permission Denied

**Symptom**: Logged-in user gets `PermissionDenied` when accessing an API.

**Troubleshooting**:

1. Check Casbin policy table: `SELECT * FROM casbin_rule WHERE v0 = 'role_name'`
2. Confirm the user's viewMenus (menu permissions) include the API corresponding to the endpoint
3. Verify the API dictionary is synced: the gateway auto-syncs from the proto Catalog at startup
4. Check that permission-contract.json includes the API declaration for the corresponding menu
5. Confirm the role has correct permissions assigned and policies are updated

### Token Validation Failure

**Symptom**: Returns `Unauthenticated` error.

**Troubleshooting**:

1. Check `Authorization` header format: `Bearer <token>` (note the space after Bearer)
2. Confirm token has not expired (default 7 days)
3. Check that the user service is running
4. Verify `jwtSignKey` config is consistent between gateway and user service

### Permission Contract Validation Failure

**Symptom**: Gateway reports permission contract errors at startup.

**Troubleshooting**:

1. Confirm frontend build artifacts include `web/dist/permission-contract.json`
2. Check that API paths in the contract file are fully uppercase
3. In dev environment, temporarily set `skipPermissionContractCheck = true`

### Missing API Classification

**Symptom**: New API that should be public requires authentication.

**Solution**: Add the corresponding path prefix to `openPack` or `justLoginPack` in `grpc_server.go`. Note that path matching uses prefix mode.

## Reference Links

- [Casbin Official Documentation](https://casbin.org/docs/overview)
- [Casbin RBAC Model](https://casbin.org/docs/rbac)
- Relevant project source code:
  - `internal/app/gateway/server/grpc_server.go` -- Auth and permission middleware, API classification
  - `internal/app/gateway/application/permission_usecase.go` -- Permission contract and API boundary checks
  - `internal/app/gateway/domain/permission/policy.go` -- Permission policy model
  - `internal/app/gateway/domain/api/api.go` -- API dictionary model
  - `internal/component/authsession/` -- Authentication session component
