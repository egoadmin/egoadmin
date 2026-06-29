# 文件上传与 Web 服务

Gateway 承载前端 SPA 静态资源分发和文件上传功能，通过 go:embed 将前端构建产物内嵌到二进制中，并提供 S3 直传和 TUS 断点续传两种上传方式。

## 概述

Gateway 的 Web 服务和文件上传功能是独立于 gRPC API 体系的 HTTP 路由。前端 SPA 通过 `go:embed` 内嵌在 gateway 二进制中，所有非 `/api` 的 GET 请求都回退到 `index.html` 实现 SPA 路由。文件上传支持两种模式：基于 multipart 的即时上传（S3 直传）和基于 TUS 协议的断点续传，文件存储在 MinIO 中，通过 CDN 签名 URL 对外提供访问。

```text
浏览器
  |-> GET /assets/*          静态资源（JS/CSS/图片）
  |-> GET /*                 SPA 回退到 index.html
  |-> POST /api/*            gRPC API（protoc-gen-go-http handler）
  |-> POST /upload           即时文件上传（S3 直传）
  |-> PATCH /files/*         TUS 断点续传
  |-> GET /file/*            CDN 签名文件访问
```

## 核心用法

### 内嵌前端 SPA

前端 `web/dist` 目录的构建产物通过 `go:embed` 嵌入到 gateway 二进制中。`StartWithFS` 函数将嵌入的文件系统注册为 SPA 静态服务。

```go
// internal/app/gateway/internal/web/web.go
func StartWithFS(fsys fs.FS, webConf config.WebConf, c *egin.Component) {
    subFS, err := fs.Sub(fsys, "web/dist")
    if err != nil {
        return
    }

    rawIndex, err := fs.ReadFile(subFS, "index.html")
    if err != nil {
        return
    }

    appConfigJS := buildAppConfigJS(webConf)
    httpFS := http.FS(subFS)
    fileServer := http.FileServer(httpFS)

    // /app-config.js 输出运行时配置
    c.GET("/app-config.js", func(ctx *gin.Context) {
        setSecurityHeaders(ctx)
        ctx.Header("Cache-Control", "no-store")
        ctx.Data(http.StatusOK, "application/javascript; charset=utf-8", appConfigJS)
    })

    // NoRoute 回退：SPA 路由
    c.NoRoute(func(ctx *gin.Context) {
        urlPath := ctx.Request.URL.Path

        // /api/* 路径不回退，返回 404
        if strings.HasPrefix(urlPath, "/api") {
            ctx.Status(http.StatusNotFound)
            return
        }

        // 仅允许 GET/HEAD 方法
        if ctx.Request.Method != http.MethodGet && ctx.Request.Method != http.MethodHead {
            ctx.Status(http.StatusNotFound)
            return
        }

        setSecurityHeaders(ctx)

        // 尝试提供静态文件
        if serveStaticFile(ctx, httpFS, fileServer, strings.TrimPrefix(urlPath, "/")) {
            return
        }

        // 回退到 index.html（SPA 路由）
        ctx.Header("Cache-Control", "no-store")
        ctx.Data(http.StatusOK, "text/html; charset=utf-8", rawIndex)
    })
}
```

### SPA 路由回退规则

| 请求路径 | 处理方式 |
|----------|----------|
| `/assets/app.js` | 匹配到静态文件，直接返回 |
| `/system/user` | 无匹配静态文件，回退到 `index.html` |
| `/api/user.v1.UserService/Login` | 不回退，走 gRPC API 路由 |
| `/admin/*` | `/admin/` 前缀路由，同样支持 SPA 回退 |

::: warning /api 前缀保护
以 `/api` 开头的路径明确不回退到 `index.html`，返回 404。这防止了 API 路径被错误地当作前端路由处理。
:::

### 运行时配置注入

`/app-config.js` 端点输出运行时配置，前端可通过 `window.__APP_CONFIG__` 读取：

```javascript
// 前端访问
console.log(window.__APP_CONFIG__);
```

```go
func buildAppConfigJS(conf config.WebConf) []byte {
    cfgJSON, _ := json.Marshal(conf)
    return []byte("window.__APP_CONFIG__=" + string(cfgJSON) + ";\n")
}
```

### Content-Security-Policy 安全头

所有静态资源和 SPA 页面都设置了严格的安全头：

```go
const contentSecurityPolicy = "default-src 'self'; base-uri 'self'; object-src 'none'; " +
    "frame-ancestors 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
    "img-src 'self' data: blob: http: https:; font-src 'self' data:; " +
    "connect-src 'self' blob: http: https: ws: wss:; media-src 'self' blob: data:; " +
    "worker-src 'self' blob:; form-action 'self'"
```

```go
func setSecurityHeaders(ctx *gin.Context) {
    ctx.Header("Content-Security-Policy", contentSecurityPolicy)
    ctx.Header("X-Content-Type-Options", "nosniff")
    ctx.Header("X-Frame-Options", "DENY")
    ctx.Header("Referrer-Policy", "strict-origin-when-cross-origin")
    ctx.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
}
```

### 即时文件上传（S3 直传）

基于 multipart 的文件上传，直接将文件存入 MinIO/S3 存储：

```go
// internal/app/gateway/internal/upload/upload.go
func WithS3Upload(s3 *xfile.S3, flake xflake.Geter, opts ...s3UploadOptions) UploadOptions {
    return UploadOptions{
        RelativePath: "/upload",
        HandleFunc: func(c *gin.Context) {
            // 1. 读取 JSON 描述信息
            // 2. 循环读取 multipart 文件 part
            // 3. 生成唯一文件名: <ext>/<date>/<hex-id-hex-timestamp>.<ext>
            // 4. 上传到 S3
            // 5. 返回文件信息
        },
    }
}
```

上传请求格式：

```bash
curl -X POST http://localhost:9001/upload \
  -H "Authorization: Bearer <token>" \
  -F 'json=[{"name":"photo.jpg","size":"102400"}]' \
  -F 'file=@photo.jpg'
```

响应格式：

```json
{
  "files": [
    {
      "filename": "jpg/2026-6-29/294e088c5cd3e564ccc1beca16a7b4817610fbfd4c3c55a.jpg",
      "size": "102400",
      "originame": "photo.jpg"
    }
  ]
}
```

### TUS 断点续传

支持 [TUS 协议](https://tus.io/) 的断点续传上传，适用于大文件上传场景。TUS 使用 S3 作为后端存储。

```go
// internal/component/upload/tus_server.go
func RegisterTusRoutes(cc *egin.Component, component *Component, opts MultipartOptions) error {
    if !component.config.Tus.Enabled {
        return nil
    }
    server, err := newTusServer(component, opts)
    if err != nil {
        return err
    }
    component.tus = server
    cc.Any(component.config.Tus.Path, server.handle)
    cc.Any(component.config.Tus.Path+"/*path", server.handle)
    return nil
}
```

TUS 上传流程：

```text
1. 客户端创建上传: POST /files -> 返回 Location: /files/<upload-id>
2. 上传分片: PATCH /files/<upload-id> -> Upload-Offset, Content-Length
3. 断点续传: HEAD /files/<upload-id> -> 返回已上传字节偏移
4. 完成上传: 所有分片上传完毕
```

### CDN 文件访问

文件下载和图片处理通过 CDN 签名 URL 提供，防止未授权访问：

```go
// server/server.go
cdncomponent.RegisterRoutes(opts.http, opts.cdn, cdncomponent.Options{
    BeforeFileHandle: func(ctx *gin.Context) (*cdncomponent.AuthContext, error) {
        auth, err := validateUploadAuth(ctx.Request.Context(), ctx.Request.Header, opts.userClient)
        if err != nil {
            return nil, err
        }
        return &cdncomponent.AuthContext{UserID: auth.UserID}, nil
    },
})
```

CDN URL 支持通过 query 参数控制下载行为：

```text
/file/jpg/2026-6-29/xxx.jpg?response-content-disposition=attachment; filename=photo.jpg
```

### 上传鉴权

所有上传和文件访问路由在处理前都先验证 Bearer token：

```go
func validateUploadAuth(ctx context.Context, header http.Header, client *userclient.Client) (*uploadcomponent.AuthContext, error) {
    token, err := extractBearerTokenFromValue(ctx, header.Get("Authorization"))
    var auth *authsession.AuthContext
    if err == nil {
        auth, err = client.InternalAuth.ValidateAccessToken(reqCtx, token)
    }
    if err != nil {
        return nil, platformi18n.ErrorFailed(reqCtx, "AuthMissingToken", nil)
    }
    return &uploadcomponent.AuthContext{UserID: auth.UserID}, nil
}
```

## 配置示例

### Web 配置

```toml
# configs/gateway/config.toml

[app.service]
webPath = "/tmp/egoadmin/core/frontend/html"  # 非嵌入模式下的前端文件路径

[app.web]
fileBaseUrl = ""           # 文件访问基础 URL（空则使用本地 CDN）
offlineOnPageLeave = false  # 离开页面时是否断开连接
```

### MinIO / S3 配置

```toml
[client.minio]
endpoint = "127.0.0.1:9000"
accessKeyID = "egoadmin"
secretAccessKey = "egoadmin123"
ssl = false
```

| 配置项 | 说明 |
|--------|------|
| `endpoint` | MinIO 服务地址 |
| `accessKeyID` | 访问密钥 ID |
| `secretAccessKey` | 访问密钥（生产环境通过环境变量覆盖） |
| `ssl` | 是否启用 HTTPS |

### CDN 配置

```toml
[component.cdn]
signSecret = "local-cdn-sign-secret"  # CDN URL 签名密钥

[client.imageProcessor]
url = "http://127.0.0.1:2853"         # 图片处理服务地址
secret = "local-image-processor-secret"
timeout = "5s"
```

### 上传组件配置

上传组件通过代码配置而非 TOML 文件。关键参数：

```go
S3UploadOption{
    maxDescSize:   4 * 1024 * 1024,        // 描述文件最大 4MB
    maxSingleSize: 1 * 1024 * 1024 * 1024, // 单文件最大 1GB
}
```

TUS 配置通过 `Component` 的 config 字段设置，包括 `Enabled`、`Path`、`ObjectPrefix`、`PartSize` 等。

## 实战示例

### 前端构建与嵌入

```bash
# 1. 构建前端
cd web && npm run build

# 2. 构建 gateway（自动嵌入 web/dist）
make build SERVICE=gateway

# 3. 启动后访问
curl http://localhost:9001/
```

前端构建产物通过 `go:embed` 指令嵌入：

```go
// 嵌入指令在包级别定义
//go:embed web/dist
var FrontendAssets embed.FS
```

### 上传图片并获取 CDN URL

```bash
# 1. 上传文件
RESP=$(curl -s -X POST http://localhost:9001/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F 'json=[{"name":"avatar.png","size":"51200"}]' \
  -F 'file=@avatar.png')

# 2. 获取文件路径
FILENAME=$(echo $RESP | jq -r '.files[0].filename')
# 输出: png/2026-6-29/294e088c5cd3e564.png

# 3. 通过 CDN 签名 URL 访问
curl "http://localhost:9001/file/$FILENAME"
```

### TUS 断点续传示例

```bash
# 1. 创建上传
curl -X POST http://localhost:9001/files/ \
  -H "Authorization: Bearer $TOKEN" \
  -H "Tus-Resumable: 1.0.0" \
  -H "Upload-Length: 10485760" \
  -H "Content-Length: 0"

# 响应头包含 Location: /files/abc123

# 2. 上传分片
curl -X PATCH http://localhost:9001/files/abc123 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Tus-Resumable: 1.0.0" \
  -H "Upload-Offset: 0" \
  -H "Content-Type: application/offset+octet-stream" \
  --data-binary @large-file.zip
```

## 工作原理

### SPA 内嵌架构

```text
编译时:
  web/dist/         --go:embed-->  egoadmin.FrontendAssets (embed.FS)
  (index.html,
   assets/app.js,    运行时:
   assets/app.css)   StartWithFS(FrontendAssets, ...)
                        |
                        v
                    http.FS(subFS) -> http.FileServer
                        |
                        v
                    NoRoute handler:
                      /assets/app.js -> 静态文件匹配 -> 直接返回
                      /system/user   -> 无匹配 -> index.html
                      /api/*         -> 不回退 -> 404
```

### 文件名生成策略

上传的文件名基于雪花 ID + 时间戳生成，按扩展名和日期组织目录：

```go
func genpath(id uint64, ext string) string {
    parentPath := strings.Replace(ext, ".", "", -1) // 如 "jpg"
    if ext == "" {
        parentPath = "other"
    }
    year, month, day := time.Now().Date()
    dataStr := fmt.Sprintf("%d-%d-%d", year, month, day)
    name := fmt.Sprintf("%x%x", id, time.Now().UnixNano())
    return path.Join(parentPath, dataStr, name+ext)
    // 例: jpg/2026-6-29/294e088c5cd3e564ccc1beca16a7b48.jpg
}
```

### 上传钩子生命周期

```text
beforeHandle(ctx)         -> 鉴权校验（验证 Bearer token）
  |                           可在此自定义权限检查
  v
MultipartReader()         -> 读取 multipart 请求
  |
  v
NextPart("json")          -> 读取 JSON 描述文件信息
  |
  v
for each file part:
  beforeUpload(part)      -> 自定义大小限制等（可选）
    |
    v
  flake.Get()             -> 生成雪花 ID
    |
    v
  s3.Upload()             -> 上传到 MinIO/S3
```

## 常见问题

### 前端页面白屏

**现象**：访问 `http://localhost:9001/` 显示空白页面。

**排查步骤**：

1. 确认前端已构建：`ls web/dist/index.html`
2. 确认 `web/dist` 已正确嵌入二进制
3. 检查浏览器控制台是否有 CSP 错误
4. 确认 `/app-config.js` 可以正常返回

### 文件上传失败

**现象**：上传请求返回错误。

**排查步骤**：

1. 确认 MinIO 正在运行：`curl http://127.0.0.1:9000/minio/health/live`
2. 检查 bucket 是否存在：`mc ls local/egoadmin`
3. 确认上传请求中包含有效的 Bearer token
4. 检查文件大小是否超过限制（单文件最大 1GB）
5. 确认 `json` part 在 `file` part 之前

### TUS 上传中断后无法续传

**现象**：断点续传时返回错误。

**排查步骤**：

1. 确认 TUS 配置已启用
2. 检查 `Tus-Resumable: 1.0.0` 请求头
3. 确认 `Upload-Offset` 与服务端记录的偏移一致
4. 检查 MinIO 中 TUS 相关的 object prefix 配置

### 静态资源加载被 CSP 拒绝

**现象**：浏览器控制台报 `Content-Security-Policy` 违规。

**解决方案**：检查前端引用的外部资源域名是否在 CSP 允许范围内。当前配置允许 `self`、`data:`、`blob:`、`http:`、`https:` 的图片加载，允许 `ws:` 和 `wss:` 的 WebSocket 连接。

### /admin 路径下前端路由不生效

**现象**：`/admin/system/user` 刷新后 404。

**排查步骤**：

1. 确认 `NoRoute` handler 中包含 `/admin/` 前缀的 SPA 回退逻辑
2. 检查前端路由配置中是否使用了 `/admin` base path

## 参考链接

- [TUS 协议规范](https://tus.io/protocols/resumable-upload.html)
- [go:embed 文档](https://pkg.go.dev/embed)
- [Content-Security-Policy 参考](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy)
- 项目内相关源码：
  - `internal/app/gateway/internal/web/web.go` — SPA 静态资源服务
  - `internal/app/gateway/internal/upload/upload.go` — S3 直传上传
  - `internal/component/upload/tus_server.go` — TUS 断点续传
  - `internal/component/cdn/` — CDN 签名分发
  - `internal/app/gateway/server/server.go` — 路由注册入口
