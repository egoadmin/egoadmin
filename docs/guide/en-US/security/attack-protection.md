# Attack Protection

EgoAdmin implements multi-layer protection at the framework and application levels, covering CORS, rate limiting, captcha, injection, XSS, CSRF, brute force, and replay attacks.

## Overview

Security defenses are distributed across three layers: gateway layer (CORS, rate limiting, request validation), component layer (LoginCrypto anti-replay, captcha), and data layer (GORM parameterized queries for injection prevention). Each layer operates independently, forming defense in depth.

```text
Request arrives
  -> CORS cross-origin check (gateway)
  -> Rate limiting (per-IP / per-user)
  -> Captcha verification (login scenarios)
  -> Request parameter validation (proto validation)
  -> LoginCrypto anti-replay
  -> GORM parameterized queries (SQL injection prevention)
  -> Output encoding (XSS prevention)
```

## Core Usage

### CORS (Cross-Origin Resource Sharing)

CORS is configured through the gateway's HTTP server:

```toml
[server.http]
enableCors = true
mode = "release"
```

When enabled, the gateway's Gin engine injects a CORS middleware:

```go
// internal/app/gateway/server/server.go
if s.config.Server.HTTP.EnableCors {
    engine.Use(cors.New(cors.Config{
        AllowOrigins:     []string{"*"},
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    }))
}
```

::: warning
Production should not use `AllowOrigins: []string{"*"}`. Configure specific frontend domain names, e.g., `["https://admin.example.com"]`.
:::

### Rate Limiting

Rate limiting is implemented via middleware for both per-IP and per-user control:

```text
Rate limiting strategy:
  -> Anonymous requests: per-IP limit
  -> Authenticated requests: per-user limit
  -> Login endpoint: strict per-IP limit (anti-brute-force)
```

Rate limiting middleware at the gateway layer:

```go
// Token bucket algorithm
func RateLimitMiddleware(rate int, burst int) gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Limit(rate), burst)
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "code": 429,
                "msg":  "Too many requests, please try again later",
            })
            c.Abort()
            return
        }
        c.Next()
    }
}
```

### Captcha

The login endpoint supports optional captcha verification:

```toml
[app.user]
useCaptcha = true
```

Login flow with captcha enabled:

```text
1. Frontend loads captcha component
2. User completes captcha
3. Frontend includes captchaId and captchaCode in login request
4. Backend verifies captcha first, then verifies password
5. Captcha failure returns error immediately, does not proceed to password verification
```

::: tip
Captcha effectively prevents automated brute force attacks. Enable it in production. Disable in development for efficiency.
:::

### SQL Injection Prevention

EgoAdmin uses GORM's parameterized queries and never concatenates raw SQL:

```go
// Safe: parameterized query
db.Where("username = ?", username).First(&user)

// Safe: GORM condition builder
db.Where("status = ? AND dept_id IN (?)", status, deptIDs).Find(&users)

// Dangerous: never do this
// db.Raw("SELECT * FROM users WHERE username = '" + username + "'")
```

::: danger
Never use string concatenation to build SQL queries. Even trusted input can be injected. Always use GORM's parameterized queries.
:::

Code review checkpoints:

```bash
# Search for potential raw SQL concatenation
grep -rn 'Raw(' internal/ --include="*.go"
grep -rn 'Exec(' internal/ --include="*.go"
grep -rn 'fmt.Sprintf.*SELECT\|INSERT\|UPDATE\|DELETE' internal/ --include="*.go"
```

### XSS Prevention

The frontend sanitizes user input:

```typescript
// Input sanitization
function sanitizeInput(input: string): string {
  return input
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#x27;')
}
```

The server sets CSP (Content Security Policy) response headers:

```text
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'
```

::: tip
EgoAdmin's frontend uses Vue 3, which automatically escapes interpolated content in templates, providing natural XSS protection. However, the `v-html` directive requires manual content safety verification.
:::

### CSRF Protection

EgoAdmin uses token-based authentication (Bearer token in header), not Cookie-based authentication, making it naturally immune to traditional CSRF attacks.

Additional protections:

```text
1. Auth token is passed in the Authorization header, not in cookies
2. Browsers do not automatically attach Authorization headers in cross-origin requests
3. SameSite cookie policy (for non-auth cookies)
```

If using cookies to store tokens (not recommended), additional configuration is needed:

```go
// Cookie security attributes
http.SetCookie(w, &http.Cookie{
    Name:     "session",
    Value:    token,
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteStrictMode,
})
```

### Brute Force Protection

Captcha and rate limiting work together to prevent brute force attacks:

```text
Protection layers:
  1. Captcha (useCaptcha = true): captcha required for each login
  2. Rate limiting: per-IP limit on login request frequency
  3. LoginCrypto challenge: each login requires fetching a challenge, increasing attack cost
  4. bcrypt delay: each password check takes ~100ms, slowing brute force attempts
```

Optional account lockout mechanism:

```go
// Login failure counting (optional implementation)
func (s *AuthService) Login(ctx context.Context, username, password string) error {
    failCount, _ := s.rdb.Get(ctx, "login_fail:"+username).Int()

    if failCount >= 5 {
        return errors.New("account locked, try again in 30 minutes")
    }

    err := s.verifyPassword(ctx, username, password)
    if err != nil {
        s.rdb.Incr(ctx, "login_fail:"+username)
        s.rdb.Expire(ctx, "login_fail:"+username, 30*time.Minute)
        return err
    }

    // Login successful, clear failure count
    s.rdb.Del(ctx, "login_fail:"+username)
    return nil
}
```

### Replay Attack Prevention

LoginCrypto's challenge-response mechanism is inherently anti-replay:

```text
Three-layer anti-replay protection:
  1. Challenge single-use: marked as consumed after use
  2. Timestamp validation: timestampSkew limits the valid window
  3. Nonce binding: encrypted payload nonce must match
```

Configuration:

```toml
[component.logincrypto]
challengeTTL = "3m0s"
timestampSkew = "2m0s"
```

### Request Parameter Validation

Proto validation tags are automatically enforced at the middleware layer:

```protobuf
// user/v1/user.proto
message AddUserRequest {
  string username = 1 [(validate.rules).string = {
    min_len: 2,
    max_len: 50
  }];
  string phone = 2 [(validate.rules).string = {
    pattern: "^1[3-9]\\d{9}$"
  }];
  string email = 3 [(validate.rules).string = {
    email: true
  }];
  int64 dept_id = 4 [(validate.rules).int64 = {
    gt: 0
  }];
}
```

Validation runs automatically at the gateway layer. Failures return a 400 error:

```go
// Request validation interceptor
func ValidationInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        if v, ok := req.(interface{ Validate() error }); ok {
            if err := v.Validate(); err != nil {
                return nil, status.Error(codes.InvalidArgument, err.Error())
            }
        }
        return handler(ctx, req)
    }
}
```

### Input Sanitization

Proto field validation tags restrict input ranges:

```protobuf
// Common validation rules
string name = 1 [(validate.rules).string = {
  min_len: 2,
  max_len: 50
}];

string phone = 2 [(validate.rules).string = {
  pattern: "^1[3-9]\\d{9}$"
}];

int32 status = 3 [(validate.rules).int32 = {
  in: [0, 1]
}];

int64 page = 4 [(validate.rules).int64 = {
  gte: 1
}];
```

## Configuration Examples

### HTTP Server Security Configuration

```toml
[server.http]
host = "0.0.0.0"
port = 9001
# Release mode: disable debug endpoints and verbose errors
mode = "release"
# Enable CORS (configure specific domains in production)
enableCors = true
ginRelativePath = "/api/*action"
```

### Login Security Configuration

```toml
[app.user]
# Enable captcha
useCaptcha = true
# Heartbeat offline detection
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660
```

### LoginCrypto Security Configuration

```toml
[component.logincrypto]
# Challenge validity period (shorter = more secure, but not too short)
challengeTTL = "3m0s"
# Timestamp drift tolerance (covers timezone differences)
timestampSkew = "2m0s"
# RSA key size
rsaKeyBits = 4096
```

## Real-World Examples

### Example 1: Production Security Hardening

```toml
[server.http]
mode = "release"         # Disable debug
enableCors = false       # CORS handled by Nginx/gateway

[app.user]
useCaptcha = true        # Enable captcha

[component.logincrypto]
challengeTTL = "2m0s"    # Shorter challenge window
timestampSkew = "1m0s"   # Shorter timestamp drift tolerance
```

### Example 2: Anti-Scraping Endpoint Protection

Configure independent rate limiting for high-frequency endpoints:

```text
Send captcha: per-IP, max 3 per minute
Password reset: per-IP, max 5 per hour
Login attempts: per-IP, max 10 per minute
```

### Example 3: Security Header Configuration

Add security response headers at the reverse proxy (Nginx) layer:

```nginx
# Nginx security headers
add_header X-Content-Type-Options nosniff;
add_header X-Frame-Options DENY;
add_header X-XSS-Protection "1; mode=block";
add_header Referrer-Policy strict-origin-when-cross-origin;
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'";
```

## How It Works

### Request Security Check Chain

```text
Client request arrives at gateway
  1. CORS preflight (OPTIONS) responds directly
  2. CORS middleware validates Origin header
  3. Rate limiting middleware checks request frequency
  4. Auth middleware extracts and validates Bearer token
  5. API classification routing (public / login-only / protected)
  6. Casbin permission check (protected APIs)
  7. Proto validation parameter check
  8. LoginCrypto challenge verification (login/password change)
  9. Business logic processing
  10. GORM parameterized queries (data access)
```

### Defense in Depth Model

```text
Layer 1: Network
  -> HTTPS transport encryption
  -> Firewall / WAF

Layer 2: Gateway
  -> CORS, rate limiting, security headers
  -> Authentication and authorization
  -> Parameter validation

Layer 3: Application
  -> LoginCrypto transport encryption
  -> Captcha
  -> Business logic validation

Layer 4: Data
  -> GORM parameterized queries
  -> bcrypt password hashing
  -> Audit logging
```

## Common Issues

### CORS configuration not working?

Confirm `enableCors = true` and the gateway HTTP server started correctly. If the frontend still reports CORS errors, check if a reverse proxy (Nginx) is involved -- the proxy layer may need separate CORS header configuration.

### Rate limiting blocking normal users?

Adjust the rate limit threshold. Login endpoints should allow 10-20 requests per minute per IP; regular APIs can be more lenient. Ensure rate limiting uses the correct client IP (obtained from the `x-forwarded-for` header).

### Captcha hurting user experience?

Only require captcha after N failed login attempts. Initial login does not need captcha; require it after 3 consecutive failures.

### How to verify SQL injection protection?

```bash
# Code review: search for raw SQL
grep -rn 'Raw(\|Exec(\|Sprintf.*SELECT\|Sprintf.*INSERT' internal/ --include="*.go"

# Security test: attempt injection
# Enter in login username field: ' OR '1'='1
# Normally handled safely by parameterized queries
```

### How to add custom validation rules in proto?

```protobuf
// Use cel expressions for custom validation
string field = 1 [(validate.rules).string = {
  cel: {
    id: "custom_rule",
    message: "Custom error message",
    expression: "this.startsWith('prefix_')"
  }
}];
```

## Reference Links

- [Authentication and Session Security](/guide/en-US/security/auth-security) -- JWT and session management
- [Password Security](/guide/en-US/security/password-security) -- LoginCrypto transport encryption
- [Security Audit](/guide/en-US/security/audit) -- Audit logging and compliance
- [Permission System](/guide/en-US/permission-system) -- Permission chain documentation
- `internal/component/logincrypto` -- LoginCrypto component source code
- `internal/app/gateway/server` -- Gateway server and middleware
