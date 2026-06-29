# LoginCrypto 登录加密

LoginCrypto 组件确保密码在网络上永远不会以明文传输。它使用 RSA-OAEP/SHA-256 加密配合一次性 challenge 机制，在客户端完成密码加密后再提交到服务端。

## 概述

EgoAdmin 的 LoginCrypto 位于 `internal/component/logincrypto`，是 `internal/component` 下的可复用组件之一。它的核心目标是：

- 防止密码在网络传输中被窃听或重放。
- 每次登录请求绑定独立的一次性 challenge，服务端验证通过后立即消费（单次使用）。
- 支持多种加密场景：登录、修改密码、编辑个人信息。
- 通过 Prometheus 指标暴露 challenge 生成和解密的延迟与错误率。

前端在提交密码前必须先调用 `GetLoginCrypto` 接口获取公钥和 challenge，然后使用浏览器原生 Web Crypto API 进行 RSA-OAEP/SHA-256 加密，最后将密文随登录请求一起发送。

## 核心用法

### 完整流程

```text
前端                              服务端
  |                                 |
  |-- GetLoginCrypto(username,      |
  |       ua, action) ------------>|
  |                                 |-- 生成/复用 RSA 密钥对
  |                                 |-- 创建 challenge（nonce + TTL）
  |                                 |-- 存储 challenge 到 Redis（JetCache）
  |<-- keyId, publicKey,            |
  |    challengeId, nonce,          |
  |    algorithm, expiresAt --------|
  |                                 |
  |-- 构造 JSON 载荷:               |
  |   {username, password,          |
  |    challengeId, nonce,          |
  |    timestamp, ua, action}       |
  |                                 |
  |-- Web Crypto RSA-OAEP 加密 ---->|  (本地操作)
  |   -> Base64 密文                |
  |                                 |
  |-- Login(passwordCipher,         |
  |       keyId, challengeId) ----->|
  |                                 |-- Consume challenge（单次消费）
  |                                 |-- 校验 keyId / username / UA / action
  |                                 |-- RSA-OAEP 解密
  |                                 |-- 校验 nonce + timestamp skew
  |<-- 登录结果 ---------------------|
```

### 前端调用示例

EgoAdmin 前端通过 `web/src/utils/login-crypto.ts` 封装了完整的加密流程：

```typescript
import { encryptPasswordPayload } from '@/utils/login-crypto'

// 登录时加密密码
const encrypted = await encryptPasswordPayload({
  username: 'admin',
  ua: clientFingerprint,
  action: 'login',
  password: 'my-secret-password',
})

// 发送登录请求
await apiUser.login({
  username: 'admin',
  ua: clientFingerprint,
  passwordCipher: encrypted.passwordCipher,
  keyId: encrypted.keyId,
  challengeId: encrypted.challengeId,
})
```

修改密码时使用 `action: 'center.edit_password'`：

```typescript
const encrypted = await encryptPasswordPayload({
  username: auth.username,
  ua: clientFingerprint,
  action: 'center.edit_password',
  oldPassword: 'current-password',
  newPassword: 'new-password',
})
```

::: tip Action 常量
前端在 `LOGIN_CRYPTO_ACTION` 中定义了所有支持的 action 值，请勿硬编码字符串。
:::

### 后端 Controller 层

Controller 层负责调用 service 层并映射 proto 字段，不包含加密业务逻辑。

**获取加密参数：**

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

**登录时解密密码：**

```go
// Controller: Login - 解密密码
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

**修改密码时解密：**

```go
// Controller: EditPassword - 解密旧密码和新密码
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

### 后端 Service 层

Service 层封装了组件的初始化和业务适配：

```go
// 初始化 LoginCrypto 组件
func NewLoginCrypto(cache *jetcache.Component, keys store.AuthCryptoKeyInterface) *logincrypto.Component {
    return logincrypto.Load("component.logincrypto").Build(
        logincrypto.WithJetCache(cache),
        logincrypto.WithKeyStore(authCryptoKeyStore{keys: keys}),
        logincrypto.WithKeyPrefix(defaults.RedisKeyPrefix),
    )
}

// 启动时预热：避免首次请求时生成 4096 位 RSA 密钥超时
func (s *UserService) WarmupLoginCrypto(ctx context.Context) error {
    if err := s.LoginCrypto.Health(ctx); err != nil {
        return fmt.Errorf("warmup login crypto: %w", err)
    }
    return nil
}
```

::: warning 必须预热
4096 位 RSA 密钥的生成可能需要数百毫秒。服务启动时应调用 `WarmupLoginCrypto` 提前生成密钥，否则第一个用户的登录请求可能因超出 gRPC deadline 而失败。
:::

## 配置说明

在服务的 TOML 配置文件中添加：

```toml
[component.logincrypto]
keyPrefix = ""                 # Redis key 前缀，可选
challengeTTL = "3m0s"          # Challenge 有效期，过期后自动失效
timestampSkew = "2m0s"         # 允许的客户端时间戳偏差
rsaKeyBits = 4096              # RSA 密钥位数，最小 2048
enableMetrics = true           # 是否启用 Prometheus 指标
```

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `keyPrefix` | string | `""` | Redis key 前缀，多租户时用于隔离 |
| `challengeTTL` | duration | `3m0s` | Challenge 有效期。前端应在此时间内完成加密 |
| `timestampSkew` | duration | `2m0s` | 允许的客户端时间戳偏差，防止时钟不同步导致失败 |
| `rsaKeyBits` | int | `4096` | RSA 密钥位数。不允许低于 2048，低于时自动回退到默认值 |
| `enableMetrics` | bool | `true` | 启用后暴露 `logincrypto_handle_total` 和 `logincrypto_handle_seconds` 指标 |

::: details 配置示例：开发环境
开发环境下可以适当放宽时间参数：

```toml
[component.logincrypto]
challengeTTL = "10m0s"
timestampSkew = "5m0s"
rsaKeyBits = 2048
enableMetrics = true
```
:::

## 实战示例

### 示例一：登录流程（完整链路）

**1. 前端发起加密请求：**

```typescript
// web/src/store/modules/user.ts
const encrypted = await encryptPasswordPayload({
  username: form.username,
  ua: getFingerprint(),
  action: 'login',
  password: form.password,
})
```

**2. Gateway 转发到 User 服务：**

```protobuf
// api/proto/user/v1/user.proto
rpc GetLoginCrypto(GetLoginCryptoRequest) returns (GetLoginCryptoResponse) {
    option (google.api.http) = {
        post: "/user.v1.UserService/GetLoginCrypto",
        body: "*"
    };
}
```

**3. User 服务生成 Challenge：**

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

**4. 前端使用 Web Crypto 加密：**

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

**5. 服务端解密并验证：**

```go
// internal/component/logincrypto/component.go
func (c *Component) DecryptPayload(ctx context.Context, req DecryptRequest) (LoginPayload, error) {
    // 1. 单次消费 challenge（GetDel 原子操作）
    record, ok, _ := c.store.Consume(ctx, c.keys.challenge(req.ChallengeID))

    // 2. 校验所有绑定字段
    if record.KeyID != req.KeyID ||
       record.Username != req.Username ||
       record.UA != req.UA ||
       record.Action != req.Action {
        return LoginPayload{}, ErrChallengeInvalid
    }

    // 3. RSA-OAEP 解密
    plain, _ := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, cipherText, nil)

    // 4. 解析 JSON 载荷
    json.Unmarshal(plain, &payload)

    // 5. 校验 nonce 和 timestamp skew
    if payload.Nonce != record.Nonce { ... }
    if now.Sub(sentAt) > c.config.TimestampSkew { ... }

    return payload, nil
}
```

### 示例二：修改密码

修改密码使用 `ActionCenterEditPassword`，载荷中包含 `oldPassword` 和 `newPassword`：

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

### 示例三：Prometheus 监控

启用 `enableMetrics = true` 后，组件暴露以下 Prometheus 指标：

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `logincrypto_handle_total` | Counter | `name`, `operation`, `code` | challenge/decrypt 操作总数 |
| `logincrypto_handle_seconds` | Histogram | `name`, `operation` | 操作延迟分布 |

可通过以下 PromQL 查询错误率：

```promql
sum(rate(logincrypto_handle_total{code="Error"}[5m])) by (operation)
/ sum(rate(logincrypto_handle_total[5m])) by (operation)
```

## 工作原理

### 密钥管理

1. RSA 密钥对按需生成（惰性初始化），首次请求 `GetLoginCrypto` 时自动创建。
2. 私钥存储在数据库的 `auth_crypto_key` 表中（通过 `KeyStore` 接口），公钥随 challenge 返回给前端。
3. 组件维护一个"活跃密钥"（`GetActive`），新 challenge 均使用当前活跃密钥。
4. 密钥位数默认 4096 位，配置中不允许低于 2048 位。

### Challenge 生命周期

1. 前端调用 `GetLoginCrypto(username, ua, action)` 获取 challenge。
2. 服务端生成 `challengeId`（18 字节随机 Base64URL）和 `nonce`（18 字节随机 Base64URL）。
3. Challenge 记录存入 Redis（JetCache），TTL 由 `challengeTTL` 控制。
4. Challenge 绑定到特定的 `username` + `ua` + `action`，不可跨用户或跨场景使用。
5. 解密时通过 `Consume`（GetDel 原子操作）读取并删除 challenge，保证单次使用。

### 载荷验证链

服务端解密后的 JSON 载荷需要通过以下全部验证：

| 验证项 | 说明 |
|--------|------|
| Challenge 存在 | Redis 中存在对应的 challenge 记录 |
| Challenge 未过期 | 当前时间在 `expiresAt` 之前 |
| KeyID 匹配 | 请求中的 `keyId` 与 challenge 记录一致 |
| Username 匹配 | 载荷中的 `username` 与 challenge 记录一致 |
| UA 匹配 | 载荷中的 `ua` 与 challenge 记录一致 |
| Action 匹配 | 载荷中的 `action` 与 challenge 记录一致 |
| Nonce 匹配 | 载荷中的 `nonce` 与 challenge 记录一致 |
| Timestamp 有效 | 载荷中的 `timestamp` 在允许的 `timestampSkew` 范围内 |

### 前端加密细节

前端使用浏览器原生 Web Crypto API，不依赖任何第三方加密库：

1. 从 PEM 格式提取 DER 字节（去除 header/footer，Base64 解码）。
2. 通过 `crypto.subtle.importKey('spki', ...)` 导入公钥，指定 `{ name: 'RSA-OAEP', hash: 'SHA-256' }`。
3. 将 `LoginPayload` JSON 序列化后 UTF-8 编码，调用 `crypto.subtle.encrypt({ name: 'RSA-OAEP' }, key, data)`。
4. 将 ArrayBuffer 密文转换为 Base64 字符串作为 `passwordCipher`。

## 常见问题

### Challenge 过期

::: danger 症状
前端加密完成后发送登录请求，服务端返回 challenge 无效。
:::

**原因：** 前端从获取 challenge 到发送登录请求的间隔超过了 `challengeTTL`（默认 3 分钟）。

**解决：**
- 检查前端是否有不必要的延迟（如等待动画、异步队列排队）。
- 如确实需要更长时间，可适当增大 `challengeTTL`，但不建议超过 5 分钟。

### 解密失败

::: danger 症状
服务端返回 `logincrypto: cipher is invalid` 错误。
:::

**原因：** 前端未正确使用 RSA-OAEP/SHA-256 算法加密。

**检查清单：**
- `crypto.subtle.importKey` 时是否指定了 `{ name: 'RSA-OAEP', hash: 'SHA-256' }`。
- `crypto.subtle.encrypt` 时是否使用了 `{ name: 'RSA-OAEP' }`。
- 公钥 PEM 是否完整（包含 `-----BEGIN PUBLIC KEY-----` 和 `-----END PUBLIC KEY-----`）。
- 密文是否正确编码为 Base64 字符串。

### 重放攻击被拦截

::: warning 症状
同一 challengeId 的第二次请求被拒绝。
:::

**说明：** 这是预期行为。每个 challenge 在 `Consume` 时已被原子删除（`GetDel`），不支持重复使用。这保证了即使攻击者截获了请求也无法重放。

### 跨用户或跨场景复用 Challenge

::: danger 症状
使用用户 A 的 challenge 发送用户 B 的登录请求，被拒绝。
:::

**说明：** Challenge 绑定了 `username`、`ua` 和 `action`，不匹配时直接返回错误。这是防止 CSRF 和跨用户攻击的安全设计。

### Proto 中禁止出现明文密码字段

::: danger 重要
API proto 中不应添加 `password`、`old_password`、`new_password` 等明文密码字段。所有密码传输必须通过 `password_cipher` + `key_id` + `challenge_id` 三元组完成。
:::

## 错误码参考

组件定义了四个错误：

| 错误 | 含义 | 常见原因 |
|------|------|----------|
| `ErrInvalidConfig` | 配置无效 | config 为 nil、challenge store 为 nil |
| `ErrChallengeInvalid` | Challenge 无效 | 过期、已消费、字段不匹配、载荷校验失败 |
| `ErrCipherInvalid` | 密文无效 | Base64 解码失败、RSA 解密失败、JSON 解析失败 |
| `ErrKeyNotFound` | 密钥未找到 | keyId 对应的私钥不存在或格式错误 |

## Action 列表

| Action 常量 | 值 | 使用场景 |
|-------------|------|----------|
| `ActionLogin` | `"login"` | 用户登录 |
| `ActionCenterEditPassword` | `"center.edit_password"` | 个人中心修改密码 |
| `ActionCenterEditInfo` | `"center.edit_info"` | 个人中心编辑信息（需要验证密码） |

::: tip
Action 为空时默认回退为 `login`。前端通过 `LOGIN_CRYPTO_ACTION` 常量管理所有 action 值。
:::

## Proto 定义参考

### GetLoginCrypto

```protobuf
message GetLoginCryptoRequest {
    string username = 1;   // 用户名/手机号，必填
    string ua = 2;         // 客户端设备标识，必填
    string action = 3;     // 加密场景，可选，默认 login
}

message GetLoginCryptoResponse {
    string key_id = 1;                        // 密钥标识
    string public_key = 2;                    // RSA 公钥 PEM
    string challenge_id = 3;                  // 一次性挑战标识
    string nonce = 4;                         // 一次性随机数
    string algorithm = 5;                     // 固定为 RSA-OAEP-SHA256
    google.protobuf.Timestamp expires_at = 6; // challenge 过期时间
}
```

### LoginRequest（密码相关字段）

```protobuf
message LoginRequest {
    string username = 1;
    string ua = 6;
    string password_cipher = 7;  // RSA-OAEP 加密后的 Base64 密文
    string key_id = 8;           // 登录加密密钥标识
    string challenge_id = 9;     // 一次性挑战标识
    // ... 其他字段（token、captcha 等）
}
```

## 参考链接

- 源码路径：`internal/component/logincrypto/`
- 前端加密工具：`web/src/utils/login-crypto.ts`
- Proto 定义：`api/proto/user/v1/user.proto`
- 组件总览：[组件系统](../components.md)
