# Password Security

EgoAdmin protects password transport with RSA-OAEP challenge-response encryption and stores passwords as bcrypt hashes, ensuring end-to-end security from client to database.

## Overview

Password security covers three layers: transport encryption, storage hashing, and password policy. The `LoginCrypto` component handles transport encryption, bcrypt handles storage hashing, and the business logic layer enforces password change policies.

```text
User enters password
  -> Fetch RSA public key and challenge
  -> Web Crypto API RSA-OAEP/SHA-256 encryption
  -> HTTPS transport ciphertext to backend
  -> Backend decrypts and validates challenge
  -> bcrypt hash comparison
```

## Core Usage

### LoginCrypto Transport Encryption

LoginCrypto uses RSA-OAEP/SHA-256 encryption with a challenge-response mechanism. Passwords never appear in plaintext during transport.

Frontend encryption flow:

```typescript
// 1. Fetch encryption parameters
const crypto = await getLoginCrypto({
  username: form.username,
  userAgent: navigator.userAgent,
  action: 'login',
})

// crypto returns:
// - publicKey: RSA public key (PEM format)
// - challengeId: one-time challenge ID
// - nonce: random value
// - keyId: key identifier

// 2. Encrypt password with Web Crypto API
const encoder = new TextEncoder()
const passwordBytes = encoder.encode(JSON.stringify({
  password: form.password,
  nonce: crypto.nonce,
  timestamp: Date.now(),
}))

const publicKey = await crypto.subtle.importKey(
  'spki',
  pemToBuffer(crypto.publicKey),
  { name: 'RSA-OAEP', hash: 'SHA-256' },
  false,
  ['encrypt']
)

const encrypted = await crypto.subtle.encrypt(
  { name: 'RSA-OAEP' },
  publicKey,
  passwordBytes
)

// 3. Submit encrypted ciphertext
await login({
  username: form.username,
  passwordCipher: arrayBufferToBase64(encrypted),
  keyId: crypto.keyId,
  challengeId: crypto.challengeId,
})
```

::: warning
The frontend must use the Web Crypto API's `RSA-OAEP` algorithm with `SHA-256` hash. Do not use other encryption schemes or implement custom encryption logic.
:::

### Backend Decryption Flow

```text
1. Receive passwordCipher, keyId, challengeId
2. Look up challenge record from Redis by challengeId
3. Validate challenge has not expired (challengeTTL)
4. Validate challenge has not been used (single-use)
5. Validate request timestamp against server time (timestampSkew)
6. Decrypt passwordCipher with RSA private key
7. Verify decrypted nonce matches the challenge nonce
8. Extract plaintext password and compare bcrypt hash
```

### bcrypt Storage Hashing

Passwords are stored as bcrypt hashes with automatic salting:

```go
// internal/app/user/application/user_app.go

// Hash password on user creation
func hashPassword(plain string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
    if err != nil {
        return "", err
    }
    return string(bytes), nil
}

// Verify password
func checkPassword(plain, hashed string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain))
    return err == nil
}
```

The default bcrypt cost is 10 (`bcrypt.DefaultCost`), balancing security and performance.

::: tip
bcrypt hashes include the algorithm version, cost, and salt. No separate salt field is needed. A typical bcrypt hash looks like `$2a$10$...`.
:::

### Default Password Policy

New users receive a default password generated from their phone number:

```go
// Internal logic
func generateDefaultPassword(phone string) string {
    // Use last 6 digits of phone number as default password
    if len(phone) >= 6 {
        return phone[len(phone)-6:]
    }
    return "123456"
}
```

Users must change their password on first login. The admin account default password is configured explicitly:

```toml
[app.user]
adminPassword = "123456"
```

::: danger
You must override `adminPassword` via environment variables in production. The admin should change the password immediately after first login.
:::

### Password Change

Changing a password requires LoginCrypto verification of the old password:

```text
1. Frontend fetches LoginCrypto parameters
2. Encrypt old password with LoginCrypto
3. Encrypt new password with LoginCrypto
4. Call change password API, submit both ciphertexts
5. Backend decrypts and verifies old password first
6. After old password verification, hash and store new password
```

### Admin Password Reset

Admins can reset any user's password without knowing the old one:

```go
// Controller layer
func (c *UserController) ResetPassword(ctx context.Context, req *pb.ResetPasswordRequest) (*pb.ResetPasswordResponse, error) {
    // Casbin permission check already completed
    newPassword := generateDefaultPassword(req.Phone)
    hashed, err := hashPassword(newPassword)
    if err != nil {
        return nil, err
    }

    err = c.userRepo.UpdatePassword(ctx, req.UserId, hashed)
    if err != nil {
        return nil, err
    }

    return &pb.ResetPasswordResponse{
        DefaultPassword: newPassword, // Return for admin to inform user
    }, nil
}
```

::: warning
After a password reset, the user should be notified to change their password. The reset action itself should be logged in the audit trail.
:::

## Configuration Examples

### LoginCrypto Configuration

```toml
[component.logincrypto]
# Challenge validity period
challengeTTL = "3m0s"
# Allowed client-server time drift
timestampSkew = "2m0s"
# RSA key size (bits)
rsaKeyBits = 4096
# Enable metrics collection
enableMetrics = true
```

### Configuration Reference

| Parameter | Default | Description |
|-----------|---------|-------------|
| `challengeTTL` | 3 minutes | Challenge expiration time. Too short fails for slow networks; too long increases replay window |
| `timestampSkew` | 2 minutes | Allowed client-server time drift. Covers timezone differences and network latency |
| `rsaKeyBits` | 4096 | RSA key size. 4096 bits provides sufficient security; do not go below 2048 |
| `enableMetrics` | true | Whether to collect LoginCrypto performance metrics |

### Production Overrides

```bash
export EGOADMIN_USER_ADMINPASSWORD="strong-admin-password"
export EGOADMIN_COMPONENT_LOGINCRYPTO_RSAKEYBITS=4096
```

## Real-World Examples

### Example 1: Complete Login Encryption Flow

```text
User enters admin / 123456
  -> Frontend calls GetLoginCrypto("admin", userAgent, "login")
  -> Backend generates RSA-4096 key pair (or uses cached pair)
  -> Backend generates challenge, stores in Redis (TTL 3 min)
  -> Returns publicKey + challengeId + nonce + keyId
  -> Frontend encrypts {"password":"123456","nonce":"...","timestamp":...} with RSA-OAEP/SHA-256
  -> Frontend calls Login("admin", base64(encrypted), keyId, challengeId)
  -> Backend verifies challenge is not expired and not used
  -> Backend verifies timestamp drift is within 2 minutes
  -> Backend decrypts with private key, verifies nonce
  -> Backend compares bcrypt hash
  -> Validation passes, issues JWT token pair
```

### Example 2: Password Change

```text
User changes password in profile settings
  -> Enter old and new passwords
  -> Fetch two sets of LoginCrypto parameters separately
  -> Encrypt old and new passwords separately
  -> Call ChangePassword(oldCipher, oldKeyId, oldChallengeId, newCipher, newKeyId, newChallengeId)
  -> Backend verifies old password first
  -> After old password passes, hash new password and store
  -> Revoke current session, require re-login
```

### Example 3: Weak Password Detection

Add weak password detection during password changes:

```go
var weakPasswords = map[string]bool{
    "123456":   true,
    "password": true,
    "admin":    true,
    "qwerty":   true,
}

func isWeakPassword(password string) bool {
    if len(password) < 8 {
        return true
    }
    if weakPasswords[password] {
        return true
    }
    // Extensible: check for username inclusion, sequential characters, etc.
    return false
}
```

## How It Works

### LoginCrypto Key Management

```text
RSA key pair lifecycle:
  1. Generate RSA-4096 key pair on first request
  2. Key pair cached in memory (configurable cache duration)
  3. Each challenge is bound to the current key's keyId
  4. On key rotation, old keys remain usable until all challenges expire
  5. challengeTTL ensures old challenges auto-expire
```

### Anti-Replay Mechanism

```text
Anti-replay protection consists of three layers:
  1. Challenge single-use: marked as consumed after use, cannot be reused
  2. Timestamp validation: timestampSkew limits the valid time window
  3. Nonce binding: decrypted nonce must match the challenge nonce
```

Even if an attacker intercepts the ciphertext, they cannot replay it because the challenge has already been consumed.

## Common Issues

### Why isn't HTTPS sufficient?

HTTPS protects the transport layer, but LoginCrypto provides application-layer encryption. Even if HTTPS is compromised (e.g., corporate proxies, certificate hijacking), passwords remain safe. This is defense in depth.

### How is bcrypt performance?

`bcrypt.DefaultCost` (10) takes about 100ms per hash on modern hardware. This latency is acceptable for login scenarios. You can adjust the cost value, but it should not go below 10.

### What challengeTTL value is recommended?

The default 3 minutes suits most scenarios. For poor network environments or password manager auto-fill (which may have delays), extend to 5 minutes. Do not exceed 10 minutes.

### RSA-2048 vs RSA-4096?

Default is RSA-4096. For performance-sensitive scenarios (high concurrent logins), RSA-2048 is acceptable and still provides sufficient security. Do not go below 2048 bits.

### Does frontend encryption need extra dependencies?

No. The browser-native Web Crypto API (`crypto.subtle`) is used, supported by all modern browsers. No additional encryption libraries are needed.

## Reference Links

- [Authentication and Session Security](/guide/en-US/security/auth-security) -- JWT and session management
- [Attack Protection](/guide/en-US/security/attack-protection) -- Anti-replay and other attack protections
- [Permission System](/guide/en-US/permission-system) -- Permission chain documentation
- `internal/component/logincrypto` -- LoginCrypto component source code
- `web/src/utils/crypto.ts` -- Frontend encryption utilities
