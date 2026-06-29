# Authentication and Session Security

EgoAdmin uses a dual-token JWT mechanism for distributed session management, backed by Redis to support multi-device login control and forced logout.

## Overview

The authentication system is built around the `authsession` component in `internal/component/authsession`. It handles issuing, refreshing, and revoking JWT tokens. The gateway layer extracts Bearer tokens from gRPC metadata to verify identity.

```text
Client request
  -> gateway gRPC interceptor extracts Authorization: Bearer <token>
  -> authsession validates token signature and expiry
  -> Redis checks session status (online/offline/revoked)
  -> API classification routing: public / login-only / protected
  -> protected APIs enter Casbin permission check
```

## Core Usage

### Token Lifecycle

The system issues two types of tokens:

| Token | Purpose | Default Lifetime |
|-------|---------|-----------------|
| access token | API request authentication | 7 days (604800 seconds) |
| refresh token | Refreshing the access token | 30 days (2592000 seconds) |

Issuance flow:

```text
1. User logs in successfully
2. Generate access token (JWT HS256 signature)
3. Generate refresh token (random UUID, stored in Redis)
4. Return token pair to client
```

### Refresh Rotation

The refresh token is invalidated after use to prevent replay attacks:

```text
Client sends refresh request with refresh token
  -> Verify refresh token exists in Redis
  -> Delete the old refresh token
  -> Issue new access + refresh token pair
  -> Return new token pair
```

::: warning
Refresh tokens are single-use. After a refresh, the old refresh token cannot be reused. If a refresh token is stolen, the original holder's refresh attempt will fail, exposing the anomaly.
:::

### JWT Signing

All JWTs are signed with the HS256 algorithm. The signing key is configured via `jwtSignKey`:

```toml
[app.user]
jwtSignKey = "CHANGE-ME-IN-PRODUCTION"
```

::: danger
You must change `jwtSignKey` in production. Using a weak or default key allows token forgery. The key should be at least 32 bytes.
:::

### Multi-Device Login Control

Control concurrent login sessions per account:

```toml
[app.user]
multiLoginEnabled = true
maxLoginClient = 2
```

Behavior:

```text
multiLoginEnabled = true, maxLoginClient = 2
  -> Same account can be logged in on up to 2 devices simultaneously
  -> When a 3rd device logs in, the earliest session is force-kicked

multiLoginEnabled = false
  -> No limit on login devices
  -> Each login creates a new session
```

The implementation maintains a session set in Redis per user. On login, the set size is checked:

```go
// internal/component/authsession/session.go
func (s *SessionManager) Login(ctx context.Context, userID int64, clientInfo string) (*TokenPair, error) {
    if s.config.MultiLoginEnabled {
        count, err := s.rdb.SCard(ctx, userSessionKey(userID)).Result()
        if err != nil {
            return nil, err
        }
        if count >= int64(s.config.MaxLoginClient) {
            // Evict the oldest session
            s.evictOldestSession(ctx, userID)
        }
    }
    // Issue new token pair and add to session set
}
```

### Heartbeat Offline Detection

Clients send periodic heartbeats, and the server records the last active time:

```toml
[app.user]
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660
```

Heartbeat mechanism:

```text
Client sends heartbeat every N seconds
  -> Server updates last_active_at timestamp in Redis
  -> No heartbeat received within heartbeatOfflineSeconds
  -> Mark user as offline
```

The default of 660 seconds (11 minutes) covers network fluctuations and client sleep scenarios.

### Forced Offline

Admins can force-kick users via the `RevokeUser` API:

```go
// Internal usage example
err := sessionManager.RevokeUser(ctx, targetUserID)
if err != nil {
    // Handle error
}
```

Forced offline revokes all active sessions for the user. The client's next request will fail token validation and redirect to the login page.

### Token Revocation

When a user logs out, both access and refresh tokens are revoked:

```text
Client requests logout
  -> Extract session ID from Bearer token
  -> Delete access token mapping from Redis
  -> Delete refresh token from Redis
  -> Remove from user session set
```

### Auth Middleware

The gateway's gRPC interceptor handles token extraction and validation:

```go
// internal/app/gateway/server/interceptor.go
func AuthInterceptor(session *authsession.SessionManager) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        md, _ := metadata.FromIncomingContext(ctx)
        tokens := md.Get("authorization")
        if len(tokens) == 0 {
            return nil, status.Error(codes.Unauthenticated, "missing token")
        }

        userID, err := session.Validate(ctx, tokens[0])
        if err != nil {
            return nil, status.Error(codes.Unauthenticated, "invalid token")
        }

        ctx = context.WithValue(ctx, ctxKeyUserID, userID)
        return handler(ctx, req)
    }
}
```

### Three-Tier API Classification

All APIs are classified into three tiers based on authentication and authorization requirements:

```text
openPack (public)
  -> No login required, no permission check
  -> Login, GetLoginCrypto, Captcha

justLoginPack (login-only)
  -> Login required, no Casbin check
  -> GetMenus, Logout, Heartbeat, GetProfile

protected
  -> Login required + Casbin permission check
  -> User management, role management, department management, etc.
```

Classification is registered at gateway startup:

```go
// internal/app/gateway/server/server.go
openPack := map[string]bool{
    "/user.v1.UserService/Login":          true,
    "/user.v1.UserService/GetLoginCrypto": true,
}

justLoginPack := map[string]bool{
    "/user.v1.UserService/GetMenus":  true,
    "/user.v1.UserService/Logout":   true,
    "/user.v1.UserService/Heartbeat": true,
}
```

APIs not in the above sets default to protected and require Casbin authorization.

### Session Store

All session data is stored in Redis to support distributed deployments:

```text
Redis key structure:
  session:{session_id}     -> {user_id, client_info, created_at, last_active}
  user_sessions:{user_id}  -> Set<session_id>
  refresh:{refresh_token}  -> session_id
```

## Configuration Examples

### Complete Auth Configuration

```toml
[app.user]
# JWT access token lifetime (seconds)
jwtExpire = 604800
# JWT refresh token lifetime (seconds)
refreshTokenExpire = 2592000
# JWT signing key (must change in production)
jwtSignKey = "CHANGE-ME-IN-PRODUCTION"
# Enable multi-device login
multiLoginEnabled = true
# Maximum concurrent login devices
maxLoginClient = 2
# Enable heartbeat offline detection
heartbeatOfflineEnabled = true
# Heartbeat timeout (seconds)
heartbeatOfflineSeconds = 660
# Revoke session on heartbeat offline
revokeSessionOnHeartbeatOffline = false
```

### Production Overrides

```bash
# Override sensitive config via environment variables
export EGOADMIN_USER_JWTSIGNKEY="your-strong-random-key-at-least-32-bytes"
export EGOADMIN_USER_MAXLOGINCLIENT=5
```

## Real-World Examples

### Example 1: Single-Device Login

Restrict users to one device only:

```toml
[app.user]
multiLoginEnabled = true
maxLoginClient = 1
```

When the user logs in on a new device, the old session is automatically cleared.

### Example 2: Production Hardening

```toml
[app.user]
jwtExpire = 3600           # 1 hour, shorter token lifetime
refreshTokenExpire = 86400 # 1 day
multiLoginEnabled = true
maxLoginClient = 3
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 300  # 5 minutes without heartbeat = offline
```

### Example 3: Admin Kicks a Problem User

```go
// In the controller
func (c *UserController) RevokeUser(ctx context.Context, req *pb.RevokeUserRequest) (*pb.RevokeUserResponse, error) {
    // Permission check already completed (Casbin)
    err := c.sessionMgr.RevokeUser(ctx, req.UserId)
    if err != nil {
        return nil, err
    }
    return &pb.RevokeUserResponse{}, nil
}
```

## How It Works

### Token Validation Flow

```text
1. Extract "authorization" header from gRPC metadata
2. Strip "Bearer " prefix to get the token string
3. Verify HS256 signature using jwtSignKey
4. Parse claims to extract user_id and session_id
5. Query Redis to confirm session has not been revoked
6. Query Redis to confirm token is not on the blacklist
7. Validation passes, write user_id into context
```

### Refresh Flow

```text
1. Client's access token expires (401 response)
2. Client requests /RefreshToken with refresh token
3. Server looks up the refresh token's session in Redis
4. Delete old refresh token and old access token
5. Issue new token pair
6. Update Redis session mapping
7. Return new token pair
```

## Common Issues

### What happens when a token expires?

The client receives a 401 response and should use the refresh token to request a new token pair. If the refresh token has also expired, the user must log in again.

### Is multi-device login limit enforced immediately?

Yes. When a new login exceeds `maxLoginClient`, the oldest session is immediately cleared. The old device will receive an authentication failure on its next request.

### What heartbeat frequency is recommended?

Client heartbeat interval should be `heartbeatOfflineSeconds / 3`. With the default config, send a heartbeat every 220 seconds to ensure at least 2 heartbeat opportunities before timeout.

### What if Redis is unavailable?

Redis unavailability will cause all authentication functions to fail. Production should use Redis Sentinel or Redis Cluster for high availability. Configure health checks at the gateway layer to return 503 when Redis is down.

### How to debug JWT issues?

```bash
# Decode the token on jwt.io to inspect claims
# Or use the Go CLI tool
go run ./cmd/user jwt decode <token>
```

## Reference Links

- [Permission System](/guide/en-US/permission-system) -- Full permission chain documentation
- [Configuration](/guide/en-US/configuration) -- Config files and environment variable overrides
- [Password Security](/guide/en-US/security/password-security) -- Password transport and storage security
- `internal/component/authsession` -- Auth session component source code
- `configs/user/config.toml` -- User service default configuration
