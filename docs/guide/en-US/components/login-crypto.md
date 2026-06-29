# LoginCrypto Login Encryption

LoginCrypto ensures passwords are never transmitted in plaintext over the network. It uses RSA-OAEP/SHA-256 encryption with one-time challenge tokens, encrypting passwords on the client side before they ever leave the browser.

## Overview

EgoAdmin's LoginCrypto lives under `internal/component/logincrypto` as a reusable component within `internal/component`. Its core responsibilities are:

- Prevent password eavesdropping and replay attacks during network transit.
- Bind each login request to a unique one-time challenge that the server consumes immediately after use.
- Support multiple encryption scenarios: login, password change, and profile editing.
- Expose Prometheus metrics for challenge generation and decryption latency and error rates.

The frontend must call `GetLoginCrypto` to obtain a public key and challenge before submitting any password, then encrypt the password client-side using the browser's native Web Crypto API (RSA-OAEP/SHA-256), and finally send the ciphertext along with the login request.

## Core Usage

### Complete Flow

```text
Frontend                           Server
  |                                  |
  |-- GetLoginCrypto(username,       |
  |       ua, action) ------------->|
  |                                  |-- Generate/reuse RSA key pair
  |                                  |-- Create challenge (nonce + TTL)
  |                                  |-- Store challenge in Redis (JetCache)
  |<-- keyId, publicKey,             |
  |    challengeId, nonce,           |
  |    algorithm, expiresAt ---------|
  |                                  |
  |-- Build JSON payload:            |
  |   {username, password,           |
  |    challengeId, nonce,           |
  |    timestamp, ua, action}        |
  |                                  |
  |-- Web Crypto RSA-OAEP encrypt -->|  (local operation)
  |   -> Base64 ciphertext           |
  |                                  |
  |-- Login(passwordCipher,          |
  |       keyId, challengeId) ------>|
  |                                  |-- Consume challenge (single use)
  |                                  |-- Validate keyId / username / UA / action
  |                                  |-- RSA-OAEP decrypt
  |                                  |-- Validate nonce + timestamp skew
  |<-- Login result ------------------|
```

### Frontend Example

The EgoAdmin frontend wraps the full encryption flow in `web/src/utils/login-crypto.ts`:

```typescript
import { encryptPasswordPayload } from '@/utils/login-crypto'

// Encrypt password for login
const encrypted = await encryptPasswordPayload({
  username: 'admin',
  ua: clientFingerprint,
  action: 'login',
  password: 'my-secret-password',
})

// Send login request
await apiUser.login({
  username: 'admin',
  ua: clientFingerprint,
  passwordCipher: encrypted.passwordCipher,
  keyId: encrypted.keyId,
  challengeId: encrypted.challengeId,
})
```

For password changes, use `action: 'center.edit_password'`:

```typescript
const encrypted = await encryptPasswordPayload({
  username: auth.username,
  ua: clientFingerprint,
  action: 'center.edit_password',
  oldPassword: 'current-password',
  newPassword: 'new-password',
})
```

::: tip Action Constants
The frontend defines all supported action values in `LOGIN_CRYPTO_ACTION`. Avoid hardcoding action strings.
:::

### Backend Controller Layer

The controller layer delegates to the service layer and maps proto fields. It contains no encryption business logic.

**Getting encryption parameters:**

```go
// Controller: GetLoginCrypto
func (s *UserGRPC) GetLoginCrypto(ctx context.Context, in *userv1.GetLoginCryptoRequest) (out *userv1.GetLoginCryptoResponse, err error) {
    out = &userv1.GetLoginCryptoResponse{}
    challenge, err := s.user.GetLoginCrypto(ctx, in.GetUsername(), in.GetUa(), in.GetAction())
    if err != nil {
        return
    }
    out.KeyId = challenge.KeyID
    out.PublicKey = challenge.PublicKey
    out.ChallengeId = challenge.ChallengeID
    out.Nonce = challenge.Nonce
    out.Algorithm = challenge.Algorithm
    out.ExpiresAt = challenge.ExpiresAt
    return
}
```

**Decrypting password on login:**

```go
// Controller: Login - decrypt password
payload, er := s.user.DecryptLoginPayload(ctx, logincrypto.DecryptRequest{
    KeyID:          in.GetKeyId(),
    ChallengeID:    in.GetChallengeId(),
    Username:       in.GetUsername(),
    UA:             in.GetUa(),
    PasswordCipher: in.GetPasswordCipher(),
    Action:         logincrypto.ActionLogin,
})
password = payload.Password
```

**Decrypting on password change:**

```go
// Controller: EditPassword - decrypt old and new passwords
payload, er := s.user.DecryptLoginPayload(ctx, logincrypto.DecryptRequest{
    KeyID:          in.GetKeyId(),
    ChallengeID:    in.GetChallengeId(),
    Username:       auth.Username,
    UA:             auth.UA,
    PasswordCipher: in.GetPasswordCipher(),
    Action:         logincrypto.ActionCenterEditPassword,
})
oldPassword := payload.OldPassword
newPassword := payload.NewPassword
```

### Backend Service Layer

The service layer wraps component initialization and business adaptation:

```go
// Initialize LoginCrypto component
func NewLoginCrypto(cache *jetcache.Component, keys store.AuthCryptoKeyInterface) *logincrypto.Component {
    return logincrypto.Load("component.logincrypto").Build(
        logincrypto.WithJetCache(cache),
        logincrypto.WithKeyStore(authCryptoKeyStore{keys: keys}),
        logincrypto.WithKeyPrefix(defaults.RedisKeyPrefix),
    )
}

// Warmup on startup: avoid generating a 4096-bit RSA key on the first request
func (s *UserService) WarmupLoginCrypto(ctx context.Context) error {
    if err := s.LoginCrypto.Health(ctx); err != nil {
        return fmt.Errorf("warmup login crypto: %w", err)
    }
    return nil
}
```

::: warning Warmup Required
Generating a 4096-bit RSA key can take several hundred milliseconds. Call `WarmupLoginCrypto` during service startup to pre-generate the key, otherwise the first user's login request may fail due to gRPC deadline exceeded.
:::

## Configuration Examples

Add the following to your service's TOML config file:

```toml
[component.logincrypto]
keyPrefix = ""                 # Redis key prefix, optional
challengeTTL = "3m0s"          # Challenge validity period
timestampSkew = "2m0s"         # Allowed client timestamp drift
rsaKeyBits = 4096              # RSA key size, minimum 2048
enableMetrics = true           # Enable Prometheus metrics
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `keyPrefix` | string | `""` | Redis key prefix, useful for multi-tenant isolation |
| `challengeTTL` | duration | `3m0s` | Challenge validity period. Frontend must complete encryption within this window |
| `timestampSkew` | duration | `2m0s` | Allowed client timestamp drift to handle clock skew |
| `rsaKeyBits` | int | `4096` | RSA key size. Values below 2048 are rejected and fall back to the default |
| `enableMetrics` | bool | `true` | When enabled, exposes `logincrypto_handle_total` and `logincrypto_handle_seconds` metrics |

::: details Example: Development Environment
For development, you can relax the timing parameters:

```toml
[component.logincrypto]
challengeTTL = "10m0s"
timestampSkew = "5m0s"
rsaKeyBits = 2048
enableMetrics = true
```
:::

## Real-World Examples

### Example 1: Login Flow (Full Chain)

**1. Frontend initiates encryption request:**

```typescript
// web/src/store/modules/user.ts
const encrypted = await encryptPasswordPayload({
  username: form.username,
  ua: getFingerprint(),
  action: 'login',
  password: form.password,
})
```

**2. Gateway forwards to User service:**

```protobuf
// api/proto/user/v1/user.proto
rpc GetLoginCrypto(GetLoginCryptoRequest) returns (GetLoginCryptoResponse) {
    option (google.api.http) = {
        post: "/user.v1.UserService/GetLoginCrypto",
        body: "*"
    };
}
```

**3. User service generates challenge:**

```go
// internal/component/logincrypto/component.go
func (c *Component) ChallengeFor(ctx context.Context, username string, ua string, action string) (Challenge, error) {
    key, _ := c.activeKey(ctx)
    challengeID, _ := randomID(18)
    nonce, _ := randomID(18)
    expiresAt := time.Now().Add(c.config.ChallengeTTL)

    record := ChallengeRecord{
        KeyID:       key.KeyID,
        Username:    username,
        UA:          ua,
        Action:      action,
        Nonce:       nonce,
        ChallengeID: challengeID,
        ExpiresAt:   expiresAt,
    }
    c.store.Set(ctx, c.keys.challenge(challengeID), record, c.config.ChallengeTTL)

    return Challenge{
        KeyID:        key.KeyID,
        PublicKeyPEM: key.PublicKeyPEM,
        ChallengeID:  challengeID,
        Nonce:        nonce,
        Algorithm:    AlgorithmRSAOAEP256,
        ExpiresAt:    expiresAt,
    }, nil
}
```

**4. Frontend encrypts with Web Crypto:**

```typescript
// web/src/utils/login-crypto.ts
const key = await importPublicKey(challenge.publicKey)
const plain = JSON.stringify({
  username: payload.username,
  password: payload.password,
  challengeId: challenge.challengeId,
  nonce: challenge.nonce,
  timestamp: Date.now(),
  ua: payload.ua,
  action: payload.action,
})
const cipher = await crypto.subtle.encrypt(
  { name: 'RSA-OAEP' },
  key,
  new TextEncoder().encode(plain),
)
```

**5. Server decrypts and validates:**

```go
// internal/component/logincrypto/component.go
func (c *Component) DecryptPayload(ctx context.Context, req DecryptRequest) (LoginPayload, error) {
    // 1. Single-use challenge consumption (atomic GetDel)
    record, ok, _ := c.store.Consume(ctx, c.keys.challenge(req.ChallengeID))

    // 2. Validate all bound fields
    if record.KeyID != req.KeyID ||
       record.Username != req.Username ||
       record.UA != req.UA ||
       record.Action != req.Action {
        return LoginPayload{}, ErrChallengeInvalid
    }

    // 3. RSA-OAEP decrypt
    plain, _ := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, cipherText, nil)

    // 4. Parse JSON payload
    json.Unmarshal(plain, &payload)

    // 5. Validate nonce and timestamp skew
    if payload.Nonce != record.Nonce { ... }
    if now.Sub(sentAt) > c.config.TimestampSkew { ... }

    return payload, nil
}
```

### Example 2: Password Change

Password changes use `ActionCenterEditPassword`. The payload contains both `oldPassword` and `newPassword`:

```go
// Controller: EditPassword
payload, er := s.user.DecryptLoginPayload(ctx, logincrypto.DecryptRequest{
    KeyID:          in.GetKeyId(),
    ChallengeID:    in.GetChallengeId(),
    Username:       auth.Username,
    UA:             auth.UA,
    PasswordCipher: in.GetPasswordCipher(),
    Action:         logincrypto.ActionCenterEditPassword,
})
if payload.OldPassword == "" || payload.NewPassword == "" {
    err = platformi18n.ErrorFailed(ctx, "InvalidLoginParams", nil)
    return
}
err = s.user.UpdateUserPassword(ctx, auth.UserID, payload.OldPassword, payload.NewPassword)
```

### Example 3: Prometheus Monitoring

With `enableMetrics = true`, the component exposes these Prometheus metrics:

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `logincrypto_handle_total` | Counter | `name`, `operation`, `code` | Total challenge/decrypt operations |
| `logincrypto_handle_seconds` | Histogram | `name`, `operation` | Operation latency distribution |

Query error rate with PromQL:

```promql
sum(rate(logincrypto_handle_total{code="Error"}[5m])) by (operation)
/ sum(rate(logincrypto_handle_total[5m])) by (operation)
```

## How It Works

### Key Management

1. RSA key pairs are generated on demand (lazy initialization), created automatically on the first `GetLoginCrypto` request.
2. Private keys are stored in the database's `auth_crypto_key` table via the `KeyStore` interface. Public keys are returned to the frontend with each challenge.
3. The component maintains a single "active key" (`GetActive`). All new challenges use the current active key.
4. Key size defaults to 4096 bits. Values below 2048 bits are rejected in configuration.

### Challenge Lifecycle

1. Frontend calls `GetLoginCrypto(username, ua, action)` to get a challenge.
2. Server generates a `challengeId` (18-byte random Base64URL) and `nonce` (18-byte random Base64URL).
3. The challenge record is stored in Redis (JetCache) with a TTL controlled by `challengeTTL`.
4. The challenge is bound to a specific `username` + `ua` + `action` and cannot be reused across users or scenarios.
5. During decryption, `Consume` (atomic GetDel) reads and deletes the challenge in one operation, guaranteeing single use.

### Payload Validation Chain

After decryption, the JSON payload must pass all of the following checks:

| Check | Description |
|-------|-------------|
| Challenge exists | A matching challenge record exists in Redis |
| Challenge not expired | Current time is before `expiresAt` |
| KeyID match | Request's `keyId` matches the challenge record |
| Username match | Payload's `username` matches the challenge record |
| UA match | Payload's `ua` matches the challenge record |
| Action match | Payload's `action` matches the challenge record |
| Nonce match | Payload's `nonce` matches the challenge record |
| Timestamp valid | Payload's `timestamp` is within the allowed `timestampSkew` range |

### Frontend Encryption Details

The frontend uses the browser's native Web Crypto API with no third-party crypto libraries:

1. Extract DER bytes from PEM format (strip header/footer, Base64 decode).
2. Import the public key via `crypto.subtle.importKey('spki', ...)` with `{ name: 'RSA-OAEP', hash: 'SHA-256' }`.
3. JSON-serialize the `LoginPayload`, UTF-8 encode it, then call `crypto.subtle.encrypt({ name: 'RSA-OAEP' }, key, data)`.
4. Convert the ArrayBuffer ciphertext to a Base64 string as `passwordCipher`.

## Common Issues

### Challenge Expired

::: danger Symptom
Frontend encrypts the password and sends the login request, but the server rejects it with an invalid challenge.
:::

**Cause:** The time between getting the challenge and sending the login request exceeds `challengeTTL` (default 3 minutes).

**Fix:**
- Check the frontend for unnecessary delays (e.g., waiting for animations, queued async operations).
- If genuinely more time is needed, increase `challengeTTL`, but avoid exceeding 5 minutes.

### Decryption Failed

::: danger Symptom
Server returns `logincrypto: cipher is invalid` error.
:::

**Cause:** The frontend is not using the correct RSA-OAEP/SHA-256 algorithm.

**Checklist:**
- Did `crypto.subtle.importKey` specify `{ name: 'RSA-OAEP', hash: 'SHA-256' }`?
- Did `crypto.subtle.encrypt` use `{ name: 'RSA-OAEP' }`?
- Is the public key PEM complete (includes `-----BEGIN PUBLIC KEY-----` and `-----END PUBLIC KEY-----`)?
- Is the ciphertext correctly encoded as a Base64 string?

### Replay Attack Blocked

::: warning Symptom
A second request with the same challengeId is rejected.
:::

**Explanation:** This is expected behavior. Each challenge is atomically deleted during `Consume` (`GetDel`) and cannot be reused. This guarantees that even if an attacker intercepts a request, they cannot replay it.

### Cross-User or Cross-Scenario Challenge Reuse

::: danger Symptom
Using user A's challenge to submit a login request for user B gets rejected.
:::

**Explanation:** Challenges are bound to `username`, `ua`, and `action`. Any mismatch returns an error immediately. This is a security design to prevent CSRF and cross-user attacks.

### No Plaintext Password Fields in Proto

::: danger Important
API proto definitions must never include plaintext password fields such as `password`, `old_password`, or `new_password`. All password transmission must use the `password_cipher` + `key_id` + `challenge_id` triplet.
:::

## Error Code Reference

The component defines four errors:

| Error | Meaning | Common Cause |
|-------|---------|--------------|
| `ErrInvalidConfig` | Invalid configuration | config is nil, challenge store is nil |
| `ErrChallengeInvalid` | Challenge invalid | Expired, already consumed, field mismatch, payload verification failed |
| `ErrCipherInvalid` | Ciphertext invalid | Base64 decode failed, RSA decrypt failed, JSON parse failed |
| `ErrKeyNotFound` | Key not found | The private key for the given keyId does not exist or has an invalid format |

## Action List

| Action Constant | Value | Usage Scenario |
|-----------------|-------|----------------|
| `ActionLogin` | `"login"` | User login |
| `ActionCenterEditPassword` | `"center.edit_password"` | Change password in personal center |
| `ActionCenterEditInfo` | `"center.edit_info"` | Edit personal information in personal center (requires password verification) |

::: tip
When Action is empty, it defaults to `login`. The frontend manages all action values through the `LOGIN_CRYPTO_ACTION` constant.
:::

## Proto Definition Reference

### GetLoginCrypto

```protobuf
message GetLoginCryptoRequest {
    string username = 1;   // Username/phone number, required
    string ua = 2;         // Client device identifier, required
    string action = 3;     // Encryption scenario, optional, defaults to login
}

message GetLoginCryptoResponse {
    string key_id = 1;                        // Key identifier
    string public_key = 2;                    // RSA public key PEM
    string challenge_id = 3;                  // One-time challenge identifier
    string nonce = 4;                         // One-time random number
    string algorithm = 5;                     // Fixed as RSA-OAEP-SHA256
    google.protobuf.Timestamp expires_at = 6; // Challenge expiration time
}
```

### LoginRequest (Password-Related Fields)

```protobuf
message LoginRequest {
    string username = 1;
    string ua = 6;
    string password_cipher = 7;  // RSA-OAEP encrypted Base64 ciphertext
    string key_id = 8;           // Login encryption key identifier
    string challenge_id = 9;     // One-time challenge identifier
    // ... other fields (token, captcha, etc.)
}
```

## Reference Links

- Source path: `internal/component/logincrypto/`
- Frontend encryption utility: `web/src/utils/login-crypto.ts`
- Proto definitions: `api/proto/user/v1/user.proto`
- Component overview: [Component System](../components.md)
