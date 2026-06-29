# 密码安全

EgoAdmin 通过 RSA-OAEP 挑战-响应机制保护密码传输，使用 bcrypt 哈希存储密码，确保密码从客户端到数据库的全链路安全。

## 概述

密码安全涉及三个层面：传输加密、存储哈希和密码策略。`LoginCrypto` 组件负责传输加密，bcrypt 负责存储哈希，业务逻辑层强制执行密码变更策略。

```text
客户端输入密码
  -> 获取 RSA 公钥和 challenge
  -> Web Crypto API RSA-OAEP/SHA-256 加密
  -> HTTPS 传输密文到后端
  -> 后端解密并校验 challenge
  -> bcrypt 比对哈希值
```

## 核心用法

### LoginCrypto 传输加密

LoginCrypto 使用 RSA-OAEP/SHA-256 加密算法实现挑战-响应机制，密码在传输过程中永远不会以明文形式出现。

前端加密流程：

```typescript
// 1. 获取加密参数
const crypto = await getLoginCrypto({
  username: form.username,
  userAgent: navigator.userAgent,
  action: 'login',
})

// crypto 返回值包含：
// - publicKey: RSA 公钥（PEM 格式）
// - challengeId: 一次性挑战 ID
// - nonce: 随机数
// - keyId: 密钥标识

// 2. 使用 Web Crypto API 加密密码
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

// 3. 提交加密后的密文
await login({
  username: form.username,
  passwordCipher: arrayBufferToBase64(encrypted),
  keyId: crypto.keyId,
  challengeId: crypto.challengeId,
})
```

::: warning
前端必须使用 Web Crypto API 的 `RSA-OAEP` 算法配合 `SHA-256` 哈希。不要使用其他加密方案或自行实现加密逻辑。
:::

### 后端解密流程

```text
1. 接收 passwordCipher、keyId、challengeId
2. 从 Redis 查询 challengeId 对应的 challenge 记录
3. 校验 challenge 是否过期（challengeTTL）
4. 校验 challenge 是否已被使用（一次性）
5. 校验请求中的 timestamp 与服务端时间差（timestampSkew）
6. 使用 RSA 私钥解密 passwordCipher
7. 验证解密后的 nonce 与 challenge 中的 nonce 一致
8. 取出明文密码进行 bcrypt 比对
```

### bcrypt 存储哈希

密码使用 bcrypt 算法哈希存储，自动加盐：

```go
// internal/app/user/application/user_app.go

// 创建用户时哈希密码
func hashPassword(plain string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
    if err != nil {
        return "", err
    }
    return string(bytes), nil
}

// 校验密码
func checkPassword(plain, hashed string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain))
    return err == nil
}
```

bcrypt 默认 cost 为 10（`bcrypt.DefaultCost`），在安全性和性能之间取得平衡。

::: tip
bcrypt 哈希值包含算法版本、cost 和盐值，无需单独存储盐字段。一个典型的 bcrypt 哈希值形如 `$2a$10$...`。
:::

### 默认密码策略

新创建的用户使用基于手机号生成的默认密码：

```go
// 内部逻辑
func generateDefaultPassword(phone string) string {
    // 取手机号后 6 位作为默认密码
    if len(phone) >= 6 {
        return phone[len(phone)-6:]
    }
    return "123456"
}
```

用户首次登录后应强制修改密码。admin 账户的默认密码通过配置指定：

```toml
[app.user]
adminPassword = "123456"
```

::: danger
生产环境必须通过环境变量覆盖 `adminPassword`，并且 admin 首次登录后立即修改密码。
:::

### 修改密码

修改密码需要通过 LoginCrypto 验证旧密码：

```text
1. 前端获取 LoginCrypto 参数
2. 使用 LoginCrypto 加密旧密码
3. 使用 LoginCrypto 加密新密码
4. 调用修改密码接口，提交两组密文
5. 后端先解密并校验旧密码
6. 旧密码校验通过后，哈希并存储新密码
```

### 管理员重置密码

管理员可以重置任意用户的密码，无需知道旧密码：

```go
// controller 层
func (c *UserController) ResetPassword(ctx context.Context, req *pb.ResetPasswordRequest) (*pb.ResetPasswordResponse, error) {
    // Casbin 权限校验已完成
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
        DefaultPassword: newPassword, // 返回给管理员告知用户
    }, nil
}
```

::: warning
管理员重置密码后，应通知用户尽快修改为自己的密码。重置操作本身应记录审计日志。
:::

## 配置示例

### LoginCrypto 配置

```toml
[component.logincrypto]
# challenge 有效期
challengeTTL = "3m0s"
# 允许的客户端-服务端时间偏差
timestampSkew = "2m0s"
# RSA 密钥长度（位）
rsaKeyBits = 4096
# 是否启用指标采集
enableMetrics = true
```

### 配置说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `challengeTTL` | 3 分钟 | challenge 过期时间。过短会导致慢网络用户失败，过长增加重放窗口 |
| `timestampSkew` | 2 分钟 | 允许的客户端与服务端时间偏差。覆盖时区差异和网络延迟 |
| `rsaKeyBits` | 4096 | RSA 密钥长度。4096 位提供足够安全性，不建议低于 2048 |
| `enableMetrics` | true | 是否采集 LoginCrypto 的性能指标 |

### 生产环境覆盖

```bash
export EGOADMIN_USER_ADMINPASSWORD="strong-admin-password"
export EGOADMIN_COMPONENT_LOGINCRYPTO_RSAKEYBITS=4096
```

## 实战场景

### 场景一：登录加密完整流程

```text
用户输入 admin / 123456
  -> 前端调用 GetLoginCrypto("admin", userAgent, "login")
  -> 后端生成 4096 位 RSA 密钥对（或使用缓存的密钥对）
  -> 后端生成 challenge，存入 Redis（TTL 3 分钟）
  -> 返回 publicKey + challengeId + nonce + keyId
  -> 前端用 RSA-OAEP/SHA-256 加密 {"password":"123456","nonce":"...","timestamp":...}
  -> 前端调用 Login("admin", base64(encrypted), keyId, challengeId)
  -> 后端验证 challenge 未过期且未使用
  -> 后端验证时间戳偏差在 2 分钟内
  -> 后端用私钥解密，验证 nonce
  -> 后端 bcrypt 比对密码
  -> 校验通过，签发 JWT token pair
```

### 场景二：密码修改

```text
用户在个人中心修改密码
  -> 输入旧密码和新密码
  -> 分别获取两组 LoginCrypto 参数
  -> 分别加密旧密码和新密码
  -> 调用 ChangePassword(oldCipher, oldKeyId, oldChallengeId, newCipher, newKeyId, newChallengeId)
  -> 后端先验证旧密码
  -> 旧密码通过后，哈希新密码并存储
  -> 撤销当前 session，要求重新登录
```

### 场景三：弱密码检测

可以在密码修改时添加弱密码检测：

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
    // 可扩展：检查是否包含用户名、连续字符等
    return false
}
```

## 工作原理

### LoginCrypto 密钥管理

```text
RSA 密钥对生命周期：
  1. 首次请求时生成 RSA-4096 密钥对
  2. 密钥对缓存在内存中（可配置缓存时间）
  3. 每个 challenge 绑定当前密钥的 keyId
  4. 密钥轮换时，旧密钥在所有 challenge 过期前仍然可用
  5. challengeTTL 保证旧 challenge 自动失效
```

### 防重放机制

```text
防重放保护由三层机制组成：
  1. challenge 一次性：使用后标记为已消费，不可重复使用
  2. 时间戳校验：timestampSkew 限制有效时间窗口
  3. nonce 绑定：解密后的 nonce 必须与 challenge 中的 nonce 匹配
```

即使攻击者截获了密文，也无法重放，因为 challenge 已被消费。

## 常见问题

### 为什么不用 HTTPS 就够了？

HTTPS 保护传输层安全，但 LoginCrypto 提供应用层加密。即使 HTTPS 被中间人攻破（如企业代理、证书劫持），密码仍然安全。这是纵深防御策略。

### bcrypt 性能如何？

`bcrypt.DefaultCost`（10）在现代硬件上每次哈希约 100ms。对于登录场景这个延迟可接受。如果需要调整，可以修改 cost 值，但不建议低于 10。

### challengeTTL 设为多长合适？

默认 3 分钟适合大多数场景。如果用户网络环境差或需要支持密码管理器自动填充（可能有延迟），可以适当延长到 5 分钟。不建议超过 10 分钟。

### RSA-2048 和 RSA-4096 如何选择？

默认使用 RSA-4096。如果性能敏感（大量并发登录），可以降为 RSA-2048，但仍提供足够安全性。不建议低于 2048 位。

### 前端加密是否需要额外依赖？

不需要。使用浏览器原生 Web Crypto API（`crypto.subtle`），所有现代浏览器都支持。不需要引入额外的加密库。

## 参考链接

- [认证与会话安全](/guide/zh-CN/security/auth-security) -- JWT 和会话管理
- [攻击防护](/guide/zh-CN/security/attack-protection) -- 防重放和其他攻击防护
- [权限系统](/guide/zh-CN/permission-system) -- 权限链路说明
- `internal/component/logincrypto` -- LoginCrypto 组件源码
- `web/src/utils/crypto.ts` -- 前端加密工具函数
