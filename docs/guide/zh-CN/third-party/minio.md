# 对象存储 MinIO

EgoAdmin 通过 MinIO 实现对象存储，支撑文件上传、TUS 断点续传和 CDN 分发等能力。

## 概述

MinIO 是 EgoAdmin 默认的对象存储后端。系统提供了两层封装：

| 层级 | 路径 | 职责 |
|------|------|------|
| Platform | `internal/platform/objectstore/` | MinIO 客户端初始化、S3 兼容接口、TUS S3 API |
| Component | `internal/component/etusupload/` | TUS 断点续传处理器、文件验证、上传钩子 |

Gateway 服务作为前端的上传代理，将文件请求转发到 MinIO。所有上传最终写入同一个 bucket，通过 key 前缀和 ULID 文件名实现命名隔离。

::: tip 为什么用 MinIO
MinIO 兼容 S3 协议，可以在本地 Docker Compose 中零成本启动，无需外部云服务。生产环境可无缝切换到 AWS S3 或其他 S3 兼容存储。
:::

## 核心用法

### 客户端初始化

Platform 层通过 `eminio.Component` 封装 MinIO 客户端，使用 Ego 的配置加载机制：

```go
// internal/platform/objectstore/minio.go

type minioConfig struct {
    Endpoint        string
    AccessKeyID     string
    SecretAccessKey  string
    Ssl             bool
    Region          string
}

func NewEMinio() *eminio.Component {
    return eminio.Load("client.minio").Build()
}
```

Wire 依赖注入自动组装：

```go
var ProviderSet = wire.NewSet(
    NewEMinio,
    NewS3,
    NewUploadObjectStore,
    NewTusS3API,
    wire.Bind(new(uploadcomponent.ObjectStore), new(*UploadObjectStore)),
    wire.Bind(new(uploadcomponent.TusS3API), new(tuss3store.S3API)),
)
```

### 对象操作

`UploadObjectStore` 实现了统一的 `ObjectStore` 接口，支持 Put / Get / Delete / Stat 四种操作：

```go
// 写入对象
err := store.Put(ctx, "avatars/user1.jpg", reader, size, uploadcomponent.PutOptions{
    ContentType: "image/jpeg",
})

// 读取对象
objReader, err := store.Get(ctx, "avatars/user1.jpg")
defer objReader.Close()

// 查询元信息
info, err := store.Stat(ctx, "avatars/user1.jpg")
// info.Key, info.Size, info.ContentType

// 删除对象
err := store.Delete(ctx, "avatars/user1.jpg")
```

::: warning 错误处理
对象不存在时返回 `uploadcomponent.ErrObjectNotFound`，而不是底层 MinIO 的 `NoSuchKey` 错误。业务代码应检查这个统一错误。
:::

### TUS 断点续传

EgoAdmin 使用 [tus](https://tus.io/) 协议实现断点续传，适用于大文件上传场景：

```go
// 组件初始化（自动从配置加载）
component := Load("component.etusupload").Build()

// 注册路由到 Gin 引擎
err := component.RegisterRoutes(engine)

// 或注册到路由组
err := component.RegisterRoutesWithGroup(apiGroup, "/tus/upload")
```

TUS 组件支持上传前和上传后的文件验证：

```go
// 自定义验证器
type MyValidator struct{}

func (v *MyValidator) ValidateBeforeUpload(ctx context.Context, metadata map[string]string) error {
    filename := metadata["filename"]
    if !isAllowedFileType(filename) {
        return fmt.Errorf("file type not allowed: %s", filename)
    }
    return nil
}

func (v *MyValidator) ValidateAfterUpload(ctx context.Context, filename string, reader io.Reader) error {
    // 检查文件头魔数，防止伪装扩展名
    return nil
}

component.RegisterValidator(&MyValidator{})
```

上传完成后，组件自动生成 ULID 作为文件 ID，将临时文件移动到最终目录，并触发 `OnAfterUpload` 钩子。

### S3 工具类

除了对象存储接口，`NewS3` 还创建了 `xfile.S3` 工具实例，支持自动建桶：

```go
func NewS3(com *eminio.Component, conf *config.Config) *xfile.S3 {
    bucketName := conf.App().BucketName
    if bucketName == "" {
        bucketName = defaults.MinioBucketName
    }
    return xfile.NewS3(com,
        xfile.WithS3AutoCreateBucket(),
        xfile.WithS3BucketName(bucketName))
}
```

## 配置示例

### MinIO 连接

```toml
[client.minio]
endpoint = "127.0.0.1:9000"
accessKeyID = "egoadmin"
secretAccessKey = "egoadmin123"
ssl = false
```

应用级 bucket 配置：

```toml
[app]
bucketName = "egoadmin"
```

::: tip Bucket 自动创建
如果配置的 bucket 不存在，`xfile.S3` 会自动创建。生产环境建议提前在 MinIO 控制台创建 bucket 并配置访问策略。
:::

### TUS 上传

```toml
[component.etusupload]
basePath = "/tus/upload"
maxSize = 1073741824           # 最大文件大小 1GB
dataDir = "./data/tus"         # TUS 临时数据目录
uploadDir = "./uploads"        # 最终上传目录
enableValidation = true
validateBeforeUpload = true
validateAfterUpload = true
allowedExtensions = ["jpg", "jpeg", "png", "gif", "pdf", "docx"]
rejectedExtensions = ["exe", "bat", "sh"]
allowedMimeTypes = []
rejectedMimeTypes = []
enableAccessLog = false
enableMetricInterceptor = true
slowLogThreshold = "1s"
enableHealthCheck = true
allowAllOrigins = true
allowedOrigins = []
```

### 环境变量覆盖

```bash
# MinIO 连接
EGOADMIN_CLIENT_MINIO_ENDPOINT=minio.prod.example.com:9000
EGOADMIN_CLIENT_MINIO_ACCESSKEYID=prod-access-key
EGOADMIN_CLIENT_MINIO_SECRETACCESSKEY=prod-secret-key
EGOADMIN_CLIENT_MINIO_SSL=true

# Bucket
EGOADMIN_APP_BUCKETNAME=egoadmin-prod
```

## 实战示例

### Gateway 上传代理

Gateway 作为前端上传代理，将 TUS 请求转发到后端存储：

```go
// 在 Gateway 初始化时注册 TUS 路由
func (s *Server) initUploadRoutes() {
    tusComponent := etusupload.Load("component.etusupload").Build()

    // 注册上传钩子：上传完成后写入数据库
    tusComponent.RegisterHook(&FileRecordHook{
        db: s.db,
    })

    tusComponent.RegisterRoutes(s.engine)
}
```

### 上传钩子：绑定业务实体

```go
type FileRecordHook struct {
    db *gorm.DB
}

func (h *FileRecordHook) OnBeforeUpload(ctx context.Context, filename string) error {
    // 上传前权限检查
    return nil
}

func (h *FileRecordHook) OnAfterUpload(ctx context.Context, info *etusupload.UploadInfo) error {
    // 将文件记录绑定到业务实体
    return h.db.WithContext(ctx).Create(&FileRecord{
        FileID:   info.FileID,
        FileName: info.FileName,
        FilePath: info.FilePath,
        FileSize: info.FileSize,
    }).Error
}

func (h *FileRecordHook) OnValidationFailed(ctx context.Context, filename string, err error) error {
    // 记录验证失败日志
    return nil
}
```

### TUS S3 API 集成

当需要将 TUS 上传直接写入 S3（而非本地文件系统）时，使用 `NewTusS3API`：

```go
func NewTusS3API() tuss3store.S3API {
    cfg := minioConfig{
        Endpoint: "localhost:9000",
        Region:   "us-east-1",
    }
    _ = econf.UnmarshalKey("client.minio", &cfg)

    awsCfg := aws.Config{
        Region:      cfg.Region,
        Credentials: credentials.NewStaticCredentialsProvider(
            cfg.AccessKeyID, cfg.SecretAccessKey, "",
        ),
    }
    return s3.NewFromConfig(awsCfg, func(options *s3.Options) {
        options.BaseEndpoint = aws.String(endpoint)
        options.UsePathStyle = true
    })
}
```

## 工作原理

```text
前端                  Gateway                 MinIO
  |                     |                      |
  |-- POST /tus/upload->|                      |
  |                     |-- TUS PATCH -------->|
  |<-- 100 Continue ----|<-- 200 OK -----------|
  |                     |                      |
  |-- POST (续传) ----->|                      |
  |                     |-- TUS PATCH -------->|
  |<-- 200 Complete ----|<-- Complete ---------|
  |                     |                      |
  |                     |-- Move to UploadDir  |
  |                     |-- OnAfterUpload hook |
```

1. 前端使用 tus 客户端库发起分块上传
2. Gateway 的 TUS handler 接收请求，写入 `dataDir` 临时目录
3. 上传完成后，组件将文件移动到 `uploadDir`，生成 ULID 文件名
4. 触发 `CompleteUploads` 通道，执行注册的上传钩子
5. 钩子可以将文件信息绑定到业务数据库

### 文件验证流程

```text
上传前验证 (PreUploadCreateCallback)
  |-- 检查文件名是否存在
  |-- 检查扩展名（allowed / rejected）
  |-- 检查 MIME 类型（allowed / rejected）
  |-- 执行自定义验证器
  |-- 执行 OnBeforeUpload 钩子

上传后验证 (PreFinishResponseCallback)
  |-- 读取已上传文件内容
  |-- 执行自定义验证器（如魔数检查）
  |-- 验证失败则终止上传并清理临时文件
```

## 常见问题

### MinIO 连接被拒绝

```text
dial tcp 127.0.0.1:9000: connect: connection refused
```

检查项：

1. MinIO 容器是否运行：`docker compose ps minio`
2. 端口是否正确映射：`docker compose port minio 9000`
3. 防火墙是否放行 9000 端口
4. 生产环境确认 DNS 解析正确，不要用 `127.0.0.1`

### Bucket 不存在

```text
The specified bucket does not exist
```

解决：

```bash
# 手动创建 bucket
mc alias set local http://127.0.0.1:9000 egoadmin egoadmin123
mc mb local/egoadmin
```

或确认配置中启用了自动建桶（默认开启）。

### TUS 上传中断后无法恢复

TUS 协议要求客户端保存 `upload-url`。如果前端页面刷新且未持久化 URL，需要重新发起上传。建议前端将 TUS URL 存入 `sessionStorage`。

### 大文件上传超时

```toml
# 调整 TUS 最大文件大小
[component.etusupload]
maxSize = 5368709120  # 5GB

# Gateway 代理也需要调整超时
[server.http]
readTimeout = "300s"
writeTimeout = "300s"
```

### 磁盘空间不足

TUS 临时文件存储在 `dataDir`，上传完成后才移动到 `uploadDir`。确保两个目录都有足够空间。建议：

- `dataDir` 至少预留最大并发上传数 x 最大文件大小
- 定期清理未完成的 TUS 上传（TUS 不会自动清理断开的上传）

## 参考链接

- [MinIO 官方文档](https://min.io/docs/)
- [TUS 协议规范](https://tus.io/protocols/resumable-upload)
- [tusd 服务端实现](https://github.com/tus/tusd)
- [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/)
- EgoAdmin 源码：`internal/platform/objectstore/`
- EgoAdmin 源码：`internal/component/etusupload/`
