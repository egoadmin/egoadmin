# AuthSession Authentication Session

AuthSession is the core authentication component providing JWT-based session management, covering token issuance, refresh, validation, revocation, multi-device login control, heartbeat-based offline detection, and forced offline.

## Overview

AuthSession lives at `internal/component/authsession` and is the authentication foundation for all protected APIs. It handles:

- **Token issuance**: Creates access token (JWT) and refresh token (opaque) on login.
- **Token refresh**: Obtains a new token pair after access token expiry; old refresh token is immediately invalidated (rotation).
- **Request validation**: gRPC middleware extracts the Bearer token from metadata, validates it, and injects `AuthContext`.
- **Session revocation**: Supports single-session, per-user, and per-workspace revocation.
- **Multi-device login**: Controls the number of concurrent sessions per user and same-device strategy.
- **Heartbeat offline**: Frontend sends periodic heartbeats; users are marked offline after timeout, with optional session revocation.

The component depends on Redis for session index storage, JetCache for session record caching, and IDGen for session ID and token ID generation. It is injected via Wire; business code depends only on the `Interface`.

```text
internal/component/authsession/
├── interface.go       # Interface definition
├── component.go       # Core implementation (Issue/Refresh/Validate/Logout/Revoke)
├── config.go          # Config and strategy enums
├── container.go       # Container + Build lifecycle
├── claims.go          # JWT Claims and AuthContext
├── middleware.go       # gRPC middleware (Server / ServerStream)
├── records.go         # SessionRecord / AccessRecord / RefreshRecord
├── errors.go          # Error codes and ecode mapping
├── cache.go           # recordCache (JetCache adapter)
├── store.go           # indexStore (Redis index)
├── keys.go            # Redis key builder
├── id.go              # IDGenerator interface
├── provider.go        # Wire ProviderSet
└── options.go         # Functional options
```

## Core Usage

### Interface

```go
type Interface interface {
    Issue(ctx context.Context, req IssueRequest) (*IssueResult, error)
    Refresh(ctx context.Context, req RefreshRequest) (*IssueResult, error)
    ValidateAccessToken(ctx context.Context, rawToken string) (*AuthContext, error)
    Logout(ctx context.Context, auth *AuthContext) error
    RevokeSession(ctx context.Context, sessionID string, reason Status) error
    RevokeUser(ctx context.Context, userID uint64, reason Status) error
    RevokeUserWorkspace(ctx context.Context, userID uint64, workspaceID uint64, reason Status) error
    ExpiresAt() time.Time
}
```

### Login (Token Issuance)

After verifying credentials, call `Issue` to create a session:

```go
issued, err := s.Auth.Issue(ctx, authsession.IssueRequest{
    UserID:   user.ID,
    Username: user.Username,
    UserType: resp.UserTyp,
    UA:       ua,
    IP:       loginip,
})
if err != nil {
    return nil, err
}
// issued.AccessToken  -> JWT returned to the frontend
// issued.RefreshToken -> Opaque refresh token returned to the frontend
// issued.ExpiresAt    -> Display expiry time for the access token
// issued.Auth         -> AuthContext for this issuance
```

`Issue` internal flow:

1. Validates request fields (UserID, Username, UA must not be empty).
2. Calls `ContextValidator` if registered (can check whether the user is disabled).
3. Calls `prepareSessionSlot` to handle multi-device login policy.
4. Generates sessionID, tokenID, and opaque refresh token via IDGen.
5. Signs JWT with HS256; Claims include uid, username, typ, ua, sid, jti.
6. Persists `SessionRecord`, `AccessRecord`, `RefreshRecord` in JetCache.
7. Maintains user session index and device session index in Redis.
8. Fires `EventRecorder` (if registered) to record the login event.

### Token Refresh

After the access token expires, the frontend calls the refresh endpoint with the refresh token:

```go
issued, err := s.Auth.Refresh(ctx, authsession.RefreshRequest{
    RefreshToken: refreshToken,
    IP:           loginip,
})
```

Refresh performs **token rotation**:

1. Validates that the `RefreshRecord` exists and its status is active.
2. Checks `SessionRecord.CurrentRefreshHash` matches (prevents replay of already-rotated tokens).
3. Marks the old access token as `StatusRotated`; marks the old refresh token as `StatusRotated`.
4. Issues a new access token and new refresh token; updates `SessionRecord`.
5. If the old refresh token is replayed, the entire session is revoked (`StatusRefreshReused`).

::: warning Token Security
Refresh tokens use a one-time-use design. Once consumed, the old value is invalidated immediately. If an old refresh token is detected as reused, the entire session is revoked and all associated tokens become invalid. This is a security measure against refresh token leakage.
:::

### Request Validation (Middleware)

The gRPC middleware automatically extracts and validates tokens. Business code retrieves authentication info via `FromContext`:

```go
auth, ok := authsession.FromContext(ctx)
if !ok {
    return platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)
}
// auth.UserID      -> User ID
// auth.Username    -> Username
// auth.UserType    -> User type
// auth.SessionID   -> Session ID
// auth.TokenID     -> Current access token ID
// auth.WorkspaceID -> Workspace ID
```

Middleware validation flow:

1. Extracts `Bearer <token>` from gRPC metadata `authorization` field.
2. Parses JWT Claims; validates signature and expiry.
3. Queries `AccessRecord` from JetCache; verifies status is active, not expired, token hash matches.
4. Queries `SessionRecord` from JetCache; verifies status is active, not expired, current access ID matches.
5. Runs `ContextValidator` (if registered).
6. Touches `SessionRecord.LastActiveAt` (if elapsed time exceeds `TouchInterval`).
7. Injects `AuthContext` into context via `NewContext(ctx, auth)`.

### Logout

```go
err = s.Auth.Logout(ctx, auth)
```

Logout calls `RevokeSession`, marking the current session and its associated access token and refresh token as `StatusLogout`.

### Force Offline

An administrator can forcefully revoke all sessions for a given user:

```go
// Revoke all sessions for a user
err = s.Auth.RevokeUser(ctx, userID, authsession.StatusRevoked)

// Revoke only sessions in a specific workspace
err = s.Auth.RevokeUserWorkspace(ctx, userID, workspaceID, authsession.StatusRevoked)
```

Revocation status enum:

| Status | Meaning | Trigger |
|--------|---------|---------|
| `StatusActive` | Normal | Session is alive |
| `StatusLogout` | Logged out | User-initiated logout |
| `StatusExpired` | Expired | Refresh token expired |
| `StatusKicked` | Kicked | Multi-device limit exceeded; oldest session evicted |
| `StatusReplaced` | Replaced | Same-device re-login |
| `StatusRevoked` | Revoked | Admin force offline |
| `StatusRotated` | Rotated | Old token marked after refresh |
| `StatusRefreshReused` | Refresh replay detected | Old refresh token reuse attack |

## Configuration Examples

### Service-Layer Config (configs/user/config.toml)

```toml
[app.user]
adminPassword = "123456"               # Initial admin password (init only)
jwtExpire = 604800                     # JWT expiry in seconds (7 days)
refreshTokenExpire = 2592000           # Refresh token expiry in seconds (30 days)
jwtSignKey = "local-egoadmin-jwt-sign-key"  # JWT HS256 signing key
useCaptcha = false                     # Enable login captcha
multiLoginEnabled = true               # Allow multiple device login
maxLoginClient = 2                     # Max concurrent sessions per user
heartbeatOfflineEnabled = true         # Enable heartbeat offline detection
heartbeatOfflineSeconds = 660          # Heartbeat timeout in seconds (11 min)
revokeSessionOnHeartbeatOffline = false # Revoke session when user goes offline
```

The service layer converts these to the component `Config`:

```go
// Internal conversion
jwtExpire (seconds)         -> AccessTokenTTL (Duration)
refreshTokenExpire (seconds)-> RefreshTokenTTL (Duration)
multiLoginEnabled           -> MultiLoginEnabled
maxLoginClient              -> MaxSessions
```

### Component-Layer Config (component.authsession)

Direct component configuration uses Duration format:

```toml
[component.authsession]
name = "default"
keyPrefix = ""
jwtSignKey = "local-egoadmin-jwt-sign-key"
accessTokenTTL = "7h"                  # Access token validity
accessTokenDisplaySkew = "30m"         # Display expiry is earlier than actual
refreshTokenTTL = "720h"               # Refresh token validity (30 days)
revokedRecordTTL = "24h"               # Retention of revoked records
touchInterval = "1m"                   # SessionRecord touch interval
multiLoginEnabled = true               # Multi-device login toggle
maxSessions = 2                        # Max concurrent sessions (0 = unlimited)
sameDeviceStrategy = "replace"         # Same-device strategy: replace / reject / allow
overflowStrategy = "revoke_oldest"     # Overflow strategy: revoke_oldest / reject
```

### Config Field Reference

| Field | Default | Description |
|-------|---------|-------------|
| `name` | `"default"` | Component instance name |
| `keyPrefix` | `""` | Redis key prefix for multi-instance isolation |
| `jwtSignKey` | `""` (required) | HS256 signing key; **must not be empty** |
| `accessTokenTTL` | `"2h"` | JWT validity period |
| `accessTokenDisplaySkew` | `"30m"` | Amount `ExpiresAt()` is earlier than actual expiry; gives the frontend a refresh window |
| `refreshTokenTTL` | `"720h"` | Refresh token validity; also the maximum session lifetime |
| `revokedRecordTTL` | `"24h"` | How long revoked records remain in cache for precise revoke-reason reporting |
| `touchInterval` | `"1m"` | Minimum interval between `LastActiveAt` updates to avoid high-frequency writes |
| `multiLoginEnabled` | `true` | When `false`, each login revokes all existing sessions |
| `maxSessions` | `0` | Max concurrent sessions; `0` means unlimited |
| `sameDeviceStrategy` | `"replace"` | Policy for sessions from the same device (same UA hash) |
| `overflowStrategy` | `"revoke_oldest"` | Policy when `maxSessions` is exceeded |

### Same-Device Strategy (SameDeviceStrategy)

| Strategy | Behavior |
|----------|----------|
| `replace` (default) | Re-login on the same device revokes the old session |
| `reject` | Login is rejected if the same device already has a session; returns `ErrSessionExists` |
| `allow` | No same-device check; allows multiple sessions on the same device |

### Overflow Strategy (OverflowStrategy)

| Strategy | Behavior |
|----------|----------|
| `revoke_oldest` (default) | When `maxSessions` is exceeded, the oldest session is evicted |
| `reject` | When `maxSessions` is exceeded, login is rejected; returns `ErrTooManySessions` |

## Real-World Examples

### Example 1: Standard Login Flow

```text
1. Frontend calls GetLoginCrypto to get RSA public key and challenge
2. Frontend encrypts password with Web Crypto RSA-OAEP/SHA-256
3. Frontend calls Login with encrypted credentials
4. Backend validates credentials -> Issue() creates session
5. Returns AccessToken + RefreshToken + ExpiresAt
6. Frontend stores tokens; subsequent requests carry Bearer token in metadata
```

### Example 2: Automatic Token Refresh

```text
1. Frontend detects LoginExpired error code in response
2. Calls Refresh endpoint with the stored RefreshToken
3. Backend performs token rotation, returns new token pair
4. Frontend updates stored tokens, retries the original request
5. If Refresh also fails (refresh token expired), redirect to login page
```

::: tip
`ExpiresAt()` returns a time that is `accessTokenDisplaySkew` (default 30 minutes) earlier than the JWT actual expiry. The frontend can proactively refresh at this point, avoiding mid-operation token expiry for the user.
:::

### Example 3: Multi-Device Login Control

```text
Config: multiLoginEnabled = true, maxLoginClient = 2

User logs in on device A -> create session 1
User logs in on device B -> create session 2 (under limit)
User logs in on device C -> revoke session 1 (oldest), create session 3

Config: multiLoginEnabled = false

User logs in on device B -> revoke ALL old sessions, create new session
```

### Example 4: Heartbeat Offline Detection

```text
1. Frontend sends heartbeat periodically (e.g., every 5 minutes)
2. Backend updates user online status and heartbeat timestamp
3. Cron job scans for users with expired heartbeats
4. Exceeds heartbeatOfflineSeconds (default 660s) without heartbeat
5. Marks user offline
6. If revokeSessionOnHeartbeatOffline = true, also revokes the session
```

Heartbeat call example:

```go
// Frontend sends heartbeat
err = s.userUseCase.MarkUserOnline(ctx, auth.UserID)
```

Cron job check:

```go
// Cron scheduled task
err = s.UserUseCase.OfflineExpiredUsers(ctx, application.OfflineExpiredCommand{
    Enabled:       conf.HeartbeatOfflineEnabled,
    Seconds:       conf.HeartbeatOfflineSeconds,
    RevokeSession: conf.RevokeSessionOnHeartbeatOffline,
})
```

### Example 5: Admin Force Offline

```text
1. Admin selects a target user in the backend panel
2. Calls RevokeUser (or RevokeUserWorkspace)
3. Component iterates all active sessions for the user, revoking each one
4. Client receives NotLogin error on next request
5. Frontend shows "Your account has been logged in elsewhere" or "Forced offline by admin"
```

### Example 6: API Auth Classification

Gateway classifies APIs into three categories; the AuthSession middleware applies accordingly:

| Category | Auth Required | Casbin Required | Examples |
|----------|---------------|-----------------|----------|
| public | No | No | Login, GetCaptcha, GetLoginCrypto |
| login-only | Yes | No | GetMenus, Logout, HeartBeatUser, GetProfile |
| protected | Yes | Yes | User management, role management, department management |

```go
// public APIs: no AuthSession middleware
// login-only APIs: AuthSession middleware, skip Casbin
// protected APIs: AuthSession middleware + Casbin check
```

## How It Works

### Token Structure

The access token is a standard JWT (HS256) with the following Claims:

```go
type Claims struct {
    UID         uint64 `json:"uid"`          // User ID
    Username    string `json:"username"`      // Username
    UserType    int32  `json:"typ"`           // User type
    UA          string `json:"ua"`            // User-Agent
    SessionID   string `json:"sid"`           // Session ID
    TokenID     string `json:"jti"`           // Token ID
    WorkspaceID uint64 `json:"workspace_id"`  // Workspace ID
    jwtv5.RegisteredClaims                     // Standard: iss, sub, exp, nbf, iat
}
```

::: warning Tokens Do Not Contain Permissions
JWT Claims store identifiers only. Roles, menus, and permissions remain server-side, queried and checked per request through Casbin and DataScope. This avoids oversized tokens and stale permission issues.
:::

The refresh token is an opaque token generated by IDGen. The server only stores its hash.

### Session Storage Model

```text
SessionRecord (JetCache)
  ├── ID, UserID, Username, UserType, UA, DeviceHash, IP, WorkspaceID
  ├── CurrentAccessID     -> points to the active AccessRecord
  ├── CurrentRefreshHash  -> points to the active RefreshRecord
  ├── Status, LoginAt, LastActiveAt, ExpiresAt
  └── RevokedAt, RevokeReason

AccessRecord (JetCache)
  ├── ID (= JWT jti), SessionID, UserID, UA
  ├── TokenHash           -> hash of the access token
  └── Status, IssuedAt, ExpiresAt, RevokedAt, RevokeReason

RefreshRecord (JetCache)
  ├── Hash                -> hash of the refresh token
  ├── SessionID, AccessID, UserID
  ├── Status, IssuedAt, ExpiresAt
  └── RotatedAt, NextTokenHash  -> after rotation, points to new refresh hash
```

Redis indexes:

```text
user_sessions:{userID}   -> Sorted Set, member=sessionId, score=loginTimeMillis
device_session:{userID}:{deviceHash} -> String, value=sessionId
```

### Request Validation Flow

```text
gRPC request arrives
  │
  ├─ metadata.ExtractIncoming(ctx).Get("authorization")
  │
  ├─ extractBearerToken() -> extract rawToken from "Bearer xxx"
  │
  ├─ parseAccessToken() -> JWT parse + HS256 signature validation
  │
  ├─ getAccess(tokenID) -> JetCache query for AccessRecord
  │   ├─ status != active -> ErrTokenRevoked
  │   ├─ expired -> ErrTokenExpired
  │   ├─ hash mismatch -> errTokenHashMismatch
  │   └─ ok
  │
  ├─ getSession(sessionID) -> JetCache query for SessionRecord
  │   ├─ status != active -> ErrSessionRevoked
  │   ├─ expired -> ErrSessionExpired
  │   ├─ currentAccessID != tokenID -> ErrInvalidToken
  │   └─ ok
  │
  ├─ ContextValidator.ValidateAuthContext() (optional)
  │
  ├─ touch LastActiveAt (if elapsed > TouchInterval)
  │
  ├─ NewContext(ctx, auth) -> AuthContext injected into context
  │
  └─ handler(ctx, req) -> business logic retrieves auth via FromContext(ctx)
```

### Error Mapping

Internal component errors are automatically mapped to API error codes:

| Internal Error | API Error | Frontend Message |
|----------------|-----------|------------------|
| `ErrMissingToken` | `Unauthenticated` | No auth token provided |
| `ErrTokenExpired` / `ErrSessionExpired` / `ErrRefreshExpired` | `LoginExpired` | Login has expired |
| `ErrInvalidToken` / `ErrTokenRevoked` / `ErrSessionRevoked` | `NotLogin` | Not logged in or invalid token |
| `ErrRefreshReused` | `NotLogin` + `LoginAbnormal` | Login abnormal |
| `StatusLogout` | `NotLogin` + `LoggedOut` | Logged out |
| `StatusKicked` | `NotLogin` + `ForcedOffline` | Forced offline |
| `StatusReplaced` | `NotLogin` + `LoginReplaced` | Replaced by new device login |

The frontend can use the `auth_status` metadata to determine the specific reason and respond accordingly.

## Common Issues

### Token expired but not auto-refreshed

**Symptom**: Frequent "login expired" prompts during user operation.

**Troubleshooting**:
- Check if `jwtExpire` is too short (default 604800 seconds = 7 days).
- Confirm the frontend implements `LoginExpired` error interception and auto-refresh logic.
- Check if the frontend proactively refreshes at the `ExpiresAt` time (rather than waiting for actual expiry).
- Review the `accessTokenDisplaySkew` setting to ensure sufficient refresh window.

### Permission not updated after role change

**Symptom**: Admin changed a user's role, but the user still sees old permissions.

**Troubleshooting**:
- Casbin policies are cached; clear the cache or wait for expiry.
- If DataScope permission snapshots use JetCache, clear those caches too.
- The quickest fix is to have the user re-login (triggers fresh permission loading).

### User still shows online after logout

**Symptom**: User has logged out, but the admin panel still shows them as online.

**Troubleshooting**:
- Confirm the heartbeat offline cron job is running.
- Check that `heartbeatOfflineEnabled` is `true`.
- Verify Redis connectivity (heartbeat status is stored in Redis).
- Check that `heartbeatOfflineSeconds` is reasonable (default 660 seconds = 11 minutes).

### Force offline not taking effect

**Symptom**: After calling `RevokeUser`, the user can still make requests.

**Troubleshooting**:
- Confirm the `reason` parameter passed to `RevokeUser` is correct (e.g., `StatusRevoked`).
- Check JetCache is functioning; revocation updates the record status in cache.
- The user will only detect the revocation on their next request (in-flight requests are unaffected).
- Check whether multiple AuthSession instances share the same Redis but have different keyPrefix values.

### Multi-device limit not evicting old sessions

**Symptom**: User's device count exceeds `maxLoginClient`, but old devices are not forced offline.

**Troubleshooting**:
- Confirm `multiLoginEnabled` is `true`.
- Check that `overflowStrategy` is `revoke_oldest` (not `reject`).
- Old devices only detect revocation on their next request.
- Verify the `maxSessions` value is correct (note: the config field is `maxSessions`, not `maxLoginClient`).

### Refresh token flagged as replay

**Symptom**: Refresh returns `LoginAbnormal` error.

**Cause**: Refresh tokens use a one-time-use design. The following scenarios trigger this error:
- Frontend sent multiple concurrent refresh requests.
- Frontend used a refresh token that was already consumed.
- Network retry caused the same refresh token to be used twice.

**Solution**:
- Implement a mutex on the frontend for refresh requests (don't send a new refresh until the previous one completes).
- Immediately update the stored refresh token after each successful refresh.

## Reference Links

- Source code: `internal/component/authsession/`
- Component system overview: [Component System](/guide/components)
- Permission system: [Permission System](/guide/permission-system)
- Runtime configuration: [Runtime Configuration](/guide/configuration)
- Graceful shutdown: [Graceful Shutdown and Lifecycle](/guide/graceful-shutdown)
- IDGen component: [IDGen ID Generation](/guide/components/idgen)
- LoginCrypto component: [LoginCrypto Login Encryption](/guide/components/login-crypto)
- ERedis component: [ERedis Cache](/guide/components/eredis)
