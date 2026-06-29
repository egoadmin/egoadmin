# File Upload and Web Service

The gateway hosts the frontend SPA static asset distribution and file upload functionality. It embeds frontend build artifacts into the binary via go:embed and provides both S3 direct upload and TUS resumable upload modes.

## Overview

The gateway's web service and file upload are HTTP routes independent of the gRPC API system. The frontend SPA is embedded in the gateway binary via `go:embed`. All non-`/api` GET requests fall back to `index.html` for SPA routing. File upload supports two modes: multipart-based instant upload (S3 direct) and TUS protocol-based resumable upload. Files are stored in MinIO and accessed via CDN-signed URLs.

```text
Browser
  |-> GET /assets/*          Static assets (JS/CSS/images)
  |-> GET /*                 SPA fallback to index.html
  |-> POST /api/*            gRPC API (via grpc-gateway)
  |-> POST /upload           Instant file upload (S3 direct)
  |-> PATCH /files/*         TUS resumable upload
  |-> GET /file/*            CDN-signed file access
```

## Core Usage

### Embedded Frontend SPA

The frontend `web/dist` directory build artifacts are embedded into the gateway binary via `go:embed`. The `StartWithFS` function registers the embedded filesystem as an SPA static service.

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

    // /app-config.js outputs runtime configuration
    c.GET("/app-config.js", func(ctx *gin.Context) {
        setSecurityHeaders(ctx)
        ctx.Header("Cache-Control", "no-store")
        ctx.Data(http.StatusOK, "application/javascript; charset=utf-8", appConfigJS)
    })

    // NoRoute fallback: SPA routing
    c.NoRoute(func(ctx *gin.Context) {
        urlPath := ctx.Request.URL.Path

        // /api/* paths do not fall back, return 404
        if strings.HasPrefix(urlPath, "/api") {
            ctx.Status(http.StatusNotFound)
            return
        }

        // Only allow GET/HEAD methods
        if ctx.Request.Method != http.MethodGet && ctx.Request.Method != http.MethodHead {
            ctx.Status(http.StatusNotFound)
            return
        }

        setSecurityHeaders(ctx)

        // Try to serve static file
        if serveStaticFile(ctx, httpFS, fileServer, strings.TrimPrefix(urlPath, "/")) {
            return
        }

        // Fall back to index.html (SPA routing)
        ctx.Header("Cache-Control", "no-store")
        ctx.Data(http.StatusOK, "text/html; charset=utf-8", rawIndex)
    })
}
```

### SPA Route Fallback Rules

| Request Path | Handling |
|-------------|----------|
| `/assets/app.js` | Matches static file, returned directly |
| `/system/user` | No matching static file, falls back to `index.html` |
| `/api/user.v1.UserService/Login` | No fallback, goes to gRPC API route |
| `/admin/*` | `/admin/` prefix routes, also support SPA fallback |

::: warning /api Prefix Protection
Paths starting with `/api` explicitly do not fall back to `index.html` and return 404. This prevents API paths from being incorrectly treated as frontend routes.
:::

### Runtime Configuration Injection

The `/app-config.js` endpoint outputs runtime configuration that the frontend can read via `window.__APP_CONFIG__`:

```javascript
// Frontend access
console.log(window.__APP_CONFIG__);
```

```go
func buildAppConfigJS(conf config.WebConf) []byte {
    cfgJSON, _ := json.Marshal(conf)
    return []byte("window.__APP_CONFIG__=" + string(cfgJSON) + ";\n")
}
```

### Content-Security-Policy Security Headers

All static assets and SPA pages have strict security headers set:

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

### Instant File Upload (S3 Direct)

Multipart-based file upload that stores files directly to MinIO/S3:

```go
// internal/app/gateway/internal/upload/upload.go
func WithS3Upload(s3 *xfile.S3, flake xflake.Geter, opts ...s3UploadOptions) UploadOptions {
    return UploadOptions{
        RelativePath: "/upload",
        HandleFunc: func(c *gin.Context) {
            // 1. Read JSON description info
            // 2. Loop through multipart file parts
            // 3. Generate unique filename: <ext>/<date>/<hex-id-hex-timestamp>.<ext>
            // 4. Upload to S3
            // 5. Return file info
        },
    }
}
```

Upload request format:

```bash
curl -X POST http://localhost:9001/upload \
  -H "Authorization: Bearer <token>" \
  -F 'json=[{"name":"photo.jpg","size":"102400"}]' \
  -F 'file=@photo.jpg'
```

Response format:

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

### TUS Resumable Upload

Supports the [TUS protocol](https://tus.io/) for resumable uploads, suitable for large file upload scenarios. TUS uses S3 as the backend storage.

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

TUS upload flow:

```text
1. Client creates upload: POST /files -> Returns Location: /files/<upload-id>
2. Upload chunk: PATCH /files/<upload-id> -> Upload-Offset, Content-Length
3. Resume: HEAD /files/<upload-id> -> Returns already-uploaded byte offset
4. Complete: All chunks uploaded
```

### CDN File Access

File downloads and image processing are provided through CDN-signed URLs to prevent unauthorized access:

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

CDN URLs support controlling download behavior via query parameters:

```text
/file/jpg/2026-6-29/xxx.jpg?response-content-disposition=attachment; filename=photo.jpg
```

### Upload Authentication

All upload and file access routes validate the Bearer token before processing:

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

## Configuration Examples

### Web Configuration

```toml
# configs/gateway/config.toml

[app.service]
webPath = "/tmp/egoadmin/core/frontend/html"  # Frontend file path in non-embedded mode

[app.web]
fileBaseUrl = ""           # File access base URL (empty uses local CDN)
offlineOnPageLeave = false  # Whether to disconnect when leaving page
```

### MinIO / S3 Configuration

```toml
[client.minio]
endpoint = "127.0.0.1:9000"
accessKeyID = "egoadmin"
secretAccessKey = "egoadmin123"
ssl = false
```

| Config Key | Description |
|------------|-------------|
| `endpoint` | MinIO service address |
| `accessKeyID` | Access key ID |
| `secretAccessKey` | Access secret (override via environment variable in production) |
| `ssl` | Whether to enable HTTPS |

### CDN Configuration

```toml
[component.cdn]
signSecret = "local-cdn-sign-secret"  # CDN URL signing secret

[client.imageProcessor]
url = "http://127.0.0.1:2853"         # Image processing service address
secret = "local-image-processor-secret"
timeout = "5s"
```

### Upload Component Configuration

The upload component is configured via code, not TOML files. Key parameters:

```go
S3UploadOption{
    maxDescSize:   4 * 1024 * 1024,        // Description file max 4MB
    maxSingleSize: 1 * 1024 * 1024 * 1024, // Single file max 1GB
}
```

TUS configuration is set through the `Component` config fields, including `Enabled`, `Path`, `ObjectPrefix`, `PartSize`, etc.

## Real-World Examples

### Frontend Build and Embedding

```bash
# 1. Build frontend
cd web && npm run build

# 2. Build gateway (auto-embeds web/dist)
make build SERVICE=gateway

# 3. Access after startup
curl http://localhost:9001/
```

Frontend build artifacts are embedded via `go:embed` directive:

```go
// Embed directive at package level
//go:embed web/dist
var FrontendAssets embed.FS
```

### Upload Image and Get CDN URL

```bash
# 1. Upload file
RESP=$(curl -s -X POST http://localhost:9001/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F 'json=[{"name":"avatar.png","size":"51200"}]' \
  -F 'file=@avatar.png')

# 2. Get file path
FILENAME=$(echo $RESP | jq -r '.files[0].filename')
# Output: png/2026-6-29/294e088c5cd3e564.png

# 3. Access via CDN-signed URL
curl "http://localhost:9001/file/$FILENAME"
```

### TUS Resumable Upload Example

```bash
# 1. Create upload
curl -X POST http://localhost:9001/files/ \
  -H "Authorization: Bearer $TOKEN" \
  -H "Tus-Resumable: 1.0.0" \
  -H "Upload-Length: 10485760" \
  -H "Content-Length: 0"

# Response header contains Location: /files/abc123

# 2. Upload chunk
curl -X PATCH http://localhost:9001/files/abc123 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Tus-Resumable: 1.0.0" \
  -H "Upload-Offset: 0" \
  -H "Content-Type: application/offset+octet-stream" \
  --data-binary @large-file.zip
```

## How It Works

### SPA Embedded Architecture

```text
Compile time:
  web/dist/         --go:embed-->  egoadmin.FrontendAssets (embed.FS)
  (index.html,
   assets/app.js,    Runtime:
   assets/app.css)   StartWithFS(FrontendAssets, ...)
                        |
                        v
                    http.FS(subFS) -> http.FileServer
                        |
                        v
                    NoRoute handler:
                      /assets/app.js -> Static file match -> Return directly
                      /system/user   -> No match -> index.html
                      /api/*         -> No fallback -> 404
```

### File Name Generation Strategy

Uploaded file names are generated based on Snowflake ID + timestamp, organized into directories by extension and date:

```go
func genpath(id uint64, ext string) string {
    parentPath := strings.Replace(ext, ".", "", -1) // e.g., "jpg"
    if ext == "" {
        parentPath = "other"
    }
    year, month, day := time.Now().Date()
    dataStr := fmt.Sprintf("%d-%d-%d", year, month, day)
    name := fmt.Sprintf("%x%x", id, time.Now().UnixNano())
    return path.Join(parentPath, dataStr, name+ext)
    // Example: jpg/2026-6-29/294e088c5cd3e564ccc1beca16a7b48.jpg
}
```

### Upload Hook Lifecycle

```text
beforeHandle(ctx)         -> Auth validation (verify Bearer token)
  |                           Custom permission checks possible here
  v
MultipartReader()         -> Read multipart request
  |
  v
NextPart("json")          -> Read JSON description file info
  |
  v
for each file part:
  beforeUpload(part)      -> Custom size limits, etc. (optional)
    |
    v
  flake.Get()             -> Generate Snowflake ID
    |
    v
  s3.Upload()             -> Upload to MinIO/S3
```

## Common Issues

### Blank Frontend Page

**Symptom**: Accessing `http://localhost:9001/` shows a blank page.

**Troubleshooting**:

1. Confirm frontend is built: `ls web/dist/index.html`
2. Confirm `web/dist` is properly embedded in the binary
3. Check browser console for CSP errors
4. Confirm `/app-config.js` returns normally

### File Upload Failure

**Symptom**: Upload request returns an error.

**Troubleshooting**:

1. Confirm MinIO is running: `curl http://127.0.0.1:9000/minio/health/live`
2. Check bucket exists: `mc ls local/egoadmin`
3. Confirm upload request includes a valid Bearer token
4. Check file size does not exceed the limit (single file max 1GB)
5. Confirm the `json` part comes before the `file` part

### TUS Upload Cannot Resume After Interruption

**Symptom**: Resumable upload returns an error.

**Troubleshooting**:

1. Confirm TUS configuration is enabled
2. Check `Tus-Resumable: 1.0.0` request header
3. Confirm `Upload-Offset` matches the offset recorded server-side
4. Check the TUS object prefix configuration in MinIO

### Static Resources Blocked by CSP

**Symptom**: Browser console reports `Content-Security-Policy` violation.

**Solution**: Check whether external resource domains referenced by the frontend are within the CSP allowed scope. The current config allows image loading from `self`, `data:`, `blob:`, `http:`, `https:`, and WebSocket connections from `ws:` and `wss:`.

### Frontend Routes Not Working Under /admin Path

**Symptom**: `/admin/system/user` returns 404 after page refresh.

**Troubleshooting**:

1. Confirm the `NoRoute` handler includes SPA fallback logic for the `/admin/` prefix
2. Check whether the frontend router config uses `/admin` as the base path

## Reference Links

- [TUS Protocol Specification](https://tus.io/protocols/resumable-upload.html)
- [go:embed Documentation](https://pkg.go.dev/embed)
- [Content-Security-Policy Reference](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy)
- Relevant project source code:
  - `internal/app/gateway/internal/web/web.go` -- SPA static asset service
  - `internal/app/gateway/internal/upload/upload.go` -- S3 direct upload
  - `internal/component/upload/tus_server.go` -- TUS resumable upload
  - `internal/component/cdn/` -- CDN signed distribution
  - `internal/app/gateway/server/server.go` -- Route registration entry
