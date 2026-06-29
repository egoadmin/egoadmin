# 攻击防护

EgoAdmin 在框架层和应用层实施多层防护，覆盖 CORS、限流、验证码、注入、XSS、CSRF、暴力破解和重放攻击等常见 Web 安全威胁。

## 概述

安全防护分布在三个层面：gateway 层（CORS、限流、请求校验）、组件层（LoginCrypto 防重放、验证码）和数据层（GORM 参数化查询防注入）。每层独立生效，形成纵深防御。

```text
请求进入
  -> CORS 跨域检查（gateway）
  -> 速率限制（per-IP / per-user）
  -> 验证码校验（登录场景）
  -> 请求参数校验（proto validation）
  -> LoginCrypto 防重放
  -> GORM 参数化查询（防 SQL 注入）
  -> 输出编码（防 XSS）
```

## 核心用法

### CORS 跨域资源共享

CORS 通过 gateway 的 HTTP 服务器配置：

```toml
[server.http]
enableCors = true
mode = "release"
```

启用后，gateway 的 Gin 引擎注入 CORS 中间件：

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
生产环境不应使用 `AllowOrigins: []string{"*"}`。应配置为具体的前端域名列表，例如 `["https://admin.example.com"]`。
:::

### 速率限制

通过中间件实现 per-IP 和 per-user 的速率限制：

```text
限流策略：
  -> 匿名请求：per-IP 限制
  -> 已认证请求：per-user 限制
  -> 登录接口：per-IP 严格限制（防暴力破解）
```

限流中间件位于 gateway 层：

```go
// 使用令牌桶算法
func RateLimitMiddleware(rate int, burst int) gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Limit(rate), burst)
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "code": 429,
                "msg":  "请求过于频繁，请稍后再试",
            })
            c.Abort()
            return
        }
        c.Next()
    }
}
```

### 验证码

登录接口支持可选的验证码校验：

```toml
[app.user]
useCaptcha = true
```

启用后的登录流程：

```text
1. 前端加载验证码组件
2. 用户完成验证码
3. 前端提交 login 请求时附带 captchaId 和 captchaCode
4. 后端先校验验证码，再校验密码
5. 验证码失败直接返回错误，不进入密码校验
```

::: tip
验证码可以有效防止自动化暴力破解攻击。生产环境建议开启。开发环境可以关闭以提升效率。
:::

### SQL 注入防护

EgoAdmin 使用 GORM 的参数化查询，不拼接原始 SQL：

```go
// 安全：参数化查询
db.Where("username = ?", username).First(&user)

// 安全：GORM 条件构建
db.Where("status = ? AND dept_id IN (?)", status, deptIDs).Find(&users)

// 危险：不要这样做
// db.Raw("SELECT * FROM users WHERE username = '" + username + "'")
```

::: danger
绝对不要使用字符串拼接构建 SQL 查询。即使输入看起来可信，也可能被注入。始终使用 GORM 的参数化查询。
:::

代码审查时检查点：

```bash
# 搜索潜在的原始 SQL 拼接
grep -rn 'Raw(' internal/ --include="*.go"
grep -rn 'Exec(' internal/ --include="*.go"
grep -rn 'fmt.Sprintf.*SELECT\|INSERT\|UPDATE\|DELETE' internal/ --include="*.go"
```

### XSS 防护

前端对用户输入进行转义和净化：

```typescript
// 输入净化
function sanitizeInput(input: string): string {
  return input
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#x27;')
}
```

服务端设置 CSP（Content Security Policy）响应头：

```text
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'
```

::: tip
EgoAdmin 的前端使用 Vue 3，Vue 模板默认对插值内容进行 HTML 转义，天然防 XSS。但使用 `v-html` 指令时需要手动确保内容安全。
:::

### CSRF 防护

EgoAdmin 使用 token-based 认证（Bearer token in header），不依赖 Cookie 传递认证信息，天然免疫传统 CSRF 攻击。

额外防护措施：

```text
1. 认证 token 在 Authorization header 中传递，不在 Cookie 中
2. 浏览器不会自动在跨域请求中附加 Authorization header
3. SameSite cookie 策略（用于非认证 Cookie）
```

如果使用 Cookie 存储 token（不推荐），需额外配置：

```go
// Cookie 安全属性
http.SetCookie(w, &http.Cookie{
    Name:     "session",
    Value:    token,
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteStrictMode,
})
```

### 暴力破解防护

结合验证码和速率限制防止暴力破解：

```text
防护层次：
  1. 验证码（useCaptcha = true）：每次登录需要验证码
  2. 速率限制：per-IP 限制登录请求频率
  3. LoginCrypto 挑战：每次登录需要获取 challenge，增加攻击成本
  4. bcrypt 延迟：每次密码校验约 100ms，减缓暴力破解速度
```

可选的账户锁定机制：

```go
// 登录失败计数（可选实现）
func (s *AuthService) Login(ctx context.Context, username, password string) error {
    failCount, _ := s.rdb.Get(ctx, "login_fail:"+username).Int()

    if failCount >= 5 {
        return errors.New("账户已锁定，请 30 分钟后再试")
    }

    err := s.verifyPassword(ctx, username, password)
    if err != nil {
        s.rdb.Incr(ctx, "login_fail:"+username)
        s.rdb.Expire(ctx, "login_fail:"+username, 30*time.Minute)
        return err
    }

    // 登录成功，清除失败计数
    s.rdb.Del(ctx, "login_fail:"+username)
    return nil
}
```

### 重放攻击防护

LoginCrypto 的挑战-响应机制天然防重放：

```text
防重放三层保护：
  1. challenge 一次性：使用后标记为已消费
  2. 时间戳校验：timestampSkew 限制有效窗口
  3. nonce 绑定：加密 payload 中的 nonce 必须匹配
```

配置：

```toml
[component.logincrypto]
challengeTTL = "3m0s"
timestampSkew = "2m0s"
```

### 请求参数校验

proto 定义中的 validation 标签在中间件层自动执行：

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

校验在 gateway 层自动执行，不通过则返回 400 错误：

```go
// 请求校验中间件
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

### 输入净化

proto 字段上的 validation 标签限制输入范围：

```protobuf
// 常用 validation 规则
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

## 配置示例

### HTTP 服务器安全配置

```toml
[server.http]
host = "0.0.0.0"
port = 9001
# 生产模式：禁用 debug 端点和详细错误信息
mode = "release"
# 启用 CORS（生产环境需配置具体域名）
enableCors = true
ginRelativePath = "/api/*action"
```

### 登录安全配置

```toml
[app.user]
# 启用验证码
useCaptcha = true
# 心跳离线检测
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660
```

### LoginCrypto 安全配置

```toml
[component.logincrypto]
# challenge 有效期（越短越安全，但不能太短）
challengeTTL = "3m0s"
# 时间戳偏差容忍（覆盖时区差异）
timestampSkew = "2m0s"
# RSA 密钥长度
rsaKeyBits = 4096
```

## 实战场景

### 场景一：生产环境安全加固

```toml
[server.http]
mode = "release"         # 禁用 debug
enableCors = false       # 由 Nginx/网关处理 CORS

[app.user]
useCaptcha = true        # 启用验证码

[component.logincrypto]
challengeTTL = "2m0s"    # 缩短 challenge 窗口
timestampSkew = "1m0s"   # 缩短时间偏差容忍
```

### 场景二：防刷接口保护

对高频接口（如发送验证码、密码重置）配置独立的限流策略：

```text
发送验证码：per-IP, 每分钟最多 3 次
密码重置：per-IP, 每小时最多 5 次
登录尝试：per-IP, 每分钟最多 10 次
```

### 场景三：安全头配置

在反向代理（Nginx）层添加安全响应头：

```nginx
# Nginx 安全头
add_header X-Content-Type-Options nosniff;
add_header X-Frame-Options DENY;
add_header X-XSS-Protection "1; mode=block";
add_header Referrer-Policy strict-origin-when-cross-origin;
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'";
```

## 工作原理

### 请求安全检查链

```text
客户端请求到达 gateway
  1. CORS 预检请求（OPTIONS）直接响应
  2. CORS 中间件校验 Origin header
  3. 速率限制中间件检查请求频率
  4. 认证中间件提取并校验 Bearer token
  5. API 分类路由（public / login-only / protected）
  6. Casbin 权限校验（protected API）
  7. proto validation 参数校验
  8. LoginCrypto challenge 校验（登录/改密场景）
  9. 业务逻辑处理
  10. GORM 参数化查询（数据访问）
```

### 纵深防御模型

```text
第 1 层：网络层
  -> HTTPS 传输加密
  -> 防火墙 / WAF

第 2 层：gateway 层
  -> CORS、速率限制、安全头
  -> 认证和权限校验
  -> 参数校验

第 3 层：应用层
  -> LoginCrypto 传输加密
  -> 验证码
  -> 业务逻辑校验

第 4 层：数据层
  -> GORM 参数化查询
  -> bcrypt 密码哈希
  -> 审计日志
```

## 常见问题

### CORS 配置不生效怎么办？

确认 `enableCors = true` 且 gateway HTTP 服务器正常启动。如果前端仍然报 CORS 错误，检查是否经过了反向代理（Nginx），代理层可能需要单独配置 CORS 头。

### 限流导致正常用户被阻断怎么办？

调整限流阈值。登录接口建议 per-IP 每分钟 10-20 次，普通 API 可以更宽松。同时确保限流基于正确的客户端 IP（通过 `x-forwarded-for` header 获取真实 IP）。

### 验证码影响用户体验怎么办？

可以只在登录失败 N 次后才要求验证码。初始登录不需要验证码，连续失败 3 次后开始要求。

### 如何验证 SQL 注入防护？

```bash
# 代码审查：搜索原始 SQL
grep -rn 'Raw(\|Exec(\|Sprintf.*SELECT\|Sprintf.*INSERT' internal/ --include="*.go"

# 安全测试：尝试注入
# 在登录用户名字段输入: ' OR '1'='1
# 正常情况下会被参数化查询安全处理
```

### proto validation 如何添加自定义规则？

```protobuf
// 使用 cel 表达式添加自定义校验
string field = 1 [(validate.rules).string = {
  cel: {
    id: "custom_rule",
    message: "自定义错误消息",
    expression: "this.startsWith('prefix_')"
  }
}];
```

## 参考链接

- [认证与会话安全](/guide/zh-CN/security/auth-security) -- JWT 和会话管理
- [密码安全](/guide/zh-CN/security/password-security) -- LoginCrypto 传输加密
- [安全审计](/guide/zh-CN/security/audit) -- 审计日志和合规
- [权限系统](/guide/zh-CN/permission-system) -- 权限链路说明
- `internal/component/logincrypto` -- LoginCrypto 组件源码
- `internal/app/gateway/server` -- gateway 服务器和中间件
