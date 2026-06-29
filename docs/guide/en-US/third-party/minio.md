# Object Storage with MinIO

EgoAdmin uses MinIO for object storage, supporting file uploads, TUS resumable uploads, and CDN distribution.

## Overview

MinIO is EgoAdmin's default object storage backend. The system provides two layers of abstraction:

| Layer | Path | Responsibility |
|-------|------|----------------|
| Platform | `internal/platform/objectstore/` | MinIO client initialization, S3-compatible interface, TUS S3 API |
| Component | `internal/component/etusupload/` | TUS resumable upload handler, file validation, upload hooks |

The Gateway service acts as an upload proxy for the frontend, forwarding file requests to MinIO. All uploads go to a single bucket, with key prefixes and ULID-based filenames providing namespace isolation.

::: tip Why MinIO
MinIO is S3-compatible, runs locally via Docker Compose with zero cost, and requires no external cloud service. In production, you can seamlessly switch to AWS S3 or any other S3-compatible storage.
:::

## Core Usage

### Client Initialization

The Platform layer wraps the MinIO client via `eminio.Component`, leveraging Ego's configuration loading mechanism:

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

Wire dependency injection assembles everything automatically:

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

### Object Operations

`UploadObjectStore` implements a unified `ObjectStore` interface with Put, Get, Delete, and Stat operations:

```go
// Write an object
err := store.Put(ctx, "avatars/user1.jpg", reader, size, uploadcomponent.PutOptions{
    ContentType: "image/jpeg",
})

// Read an object
objReader, err := store.Get(ctx, "avatars/user1.jpg")
defer objReader.Close()

// Query metadata
info, err := store.Stat(ctx, "avatars/user1.jpg")
// info.Key, info.Size, info.ContentType

// Delete an object
err := store.Delete(ctx, "avatars/user1.jpg")
```

::: warning Error Handling
When an object does not exist, the component returns `uploadcomponent.ErrObjectNotFound` instead of the underlying MinIO `NoSuchKey` error. Business code should check for this unified error.
:::

### TUS Resumable Uploads

EgoAdmin implements resumable uploads using the [tus](https://tus.io/) protocol, suitable for large file upload scenarios:

```go
// Component initialization (loads config automatically)
component := Load("component.etusupload").Build()

// Register routes on the Gin engine
err := component.RegisterRoutes(engine)

// Or register within a route group
err := component.RegisterRoutesWithGroup(apiGroup, "/tus/upload")
```

The TUS component supports pre-upload and post-upload file validation:

```go
// Custom validator
type MyValidator struct{}

func (v *MyValidator) ValidateBeforeUpload(ctx context.Context, metadata map[string]string) error {
    filename := metadata["filename"]
    if !isAllowedFileType(filename) {
        return fmt.Errorf("file type not allowed: %s", filename)
    }
    return nil
}

func (v *MyValidator) ValidateAfterUpload(ctx context.Context, filename string, reader io.Reader) error {
    // Check file magic bytes to prevent disguised extensions
    return nil
}

component.RegisterValidator(&MyValidator{})
```

After upload completes, the component automatically generates a ULID as the file ID, moves the temporary file to the final directory, and triggers the `OnAfterUpload` hook.

### S3 Utility

In addition to the object store interface, `NewS3` creates an `xfile.S3` utility instance that supports automatic bucket creation:

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

## Configuration Examples

### MinIO Connection

```toml
[client.minio]
endpoint = "127.0.0.1:9000"
accessKeyID = "egoadmin"
secretAccessKey = "egoadmin123"
ssl = false
```

Application-level bucket configuration:

```toml
[app]
bucketName = "egoadmin"
```

::: tip Bucket Auto-Creation
If the configured bucket does not exist, `xfile.S3` creates it automatically. In production, create the bucket in advance via the MinIO console and configure access policies.
:::

### TUS Upload

```toml
[component.etusupload]
basePath = "/tus/upload"
maxSize = 1073741824           # Max file size 1GB
dataDir = "./data/tus"         # TUS temporary data directory
uploadDir = "./uploads"        # Final upload directory
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

### Environment Variable Overrides

```bash
# MinIO connection
EGOADMIN_CLIENT_MINIO_ENDPOINT=minio.prod.example.com:9000
EGOADMIN_CLIENT_MINIO_ACCESSKEYID=prod-access-key
EGOADMIN_CLIENT_MINIO_SECRETACCESSKEY=prod-secret-key
EGOADMIN_CLIENT_MINIO_SSL=true

# Bucket
EGOADMIN_APP_BUCKETNAME=egoadmin-prod
```

## Real-World Examples

### Gateway Upload Proxy

The Gateway acts as the frontend upload proxy, forwarding TUS requests to the backend storage:

```go
// Register TUS routes during Gateway initialization
func (s *Server) initUploadRoutes() {
    tusComponent := etusupload.Load("component.etusupload").Build()

    // Register upload hook: write to database after upload completes
    tusComponent.RegisterHook(&FileRecordHook{
        db: s.db,
    })

    tusComponent.RegisterRoutes(s.engine)
}
```

### Upload Hook: Binding Business Entities

```go
type FileRecordHook struct {
    db *gorm.DB
}

func (h *FileRecordHook) OnBeforeUpload(ctx context.Context, filename string) error {
    // Permission check before upload
    return nil
}

func (h *FileRecordHook) OnAfterUpload(ctx context.Context, info *etusupload.UploadInfo) error {
    // Bind file record to a business entity
    return h.db.WithContext(ctx).Create(&FileRecord{
        FileID:   info.FileID,
        FileName: info.FileName,
        FilePath: info.FilePath,
        FileSize: info.FileSize,
    }).Error
}

func (h *FileRecordHook) OnValidationFailed(ctx context.Context, filename string, err error) error {
    // Log validation failure
    return nil
}
```

### TUS S3 API Integration

When you need TUS uploads to write directly to S3 (rather than the local filesystem), use `NewTusS3API`:

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

## How It Works

```text
Frontend              Gateway                 MinIO
  |                     |                      |
  |-- POST /tus/upload->|                      |
  |                     |-- TUS PATCH -------->|
  |<-- 100 Continue ----|<-- 200 OK -----------|
  |                     |                      |
  |-- POST (resume) --->|                      |
  |                     |-- TUS PATCH -------->|
  |<-- 200 Complete ----|<-- Complete ---------|
  |                     |                      |
  |                     |-- Move to UploadDir  |
  |                     |-- OnAfterUpload hook |
```

1. The frontend initiates a chunked upload using a tus client library
2. The Gateway's TUS handler receives the request and writes to the `dataDir` temporary directory
3. After upload completes, the component moves the file to `uploadDir` and generates a ULID-based filename
4. The `CompleteUploads` channel is triggered, executing registered upload hooks
5. Hooks can bind file information to the business database

### File Validation Flow

```text
Pre-upload validation (PreUploadCreateCallback)
  |-- Check that filename exists
  |-- Check extension (allowed / rejected)
  |-- Check MIME type (allowed / rejected)
  |-- Execute custom validator
  |-- Execute OnBeforeUpload hooks

Post-upload validation (PreFinishResponseCallback)
  |-- Read uploaded file content
  |-- Execute custom validator (e.g., magic byte check)
  |-- On failure, terminate upload and clean up temp files
```

## Common Issues

### MinIO Connection Refused

```text
dial tcp 127.0.0.1:9000: connect: connection refused
```

Check these items:

1. Is the MinIO container running? `docker compose ps minio`
2. Is the port mapped correctly? `docker compose port minio 9000`
3. Is port 9000 open in the firewall?
4. In production, verify DNS resolution and do not use `127.0.0.1`

### Bucket Does Not Exist

```text
The specified bucket does not exist
```

Solution:

```bash
# Create bucket manually
mc alias set local http://127.0.0.1:9000 egoadmin egoadmin123
mc mb local/egoadmin
```

Or verify that auto-bucket-creation is enabled in configuration (enabled by default).

### TUS Upload Cannot Resume After Interruption

The TUS protocol requires the client to persist the `upload-url`. If the frontend page refreshes without persisting the URL, the upload must be restarted. Store the TUS URL in `sessionStorage` on the frontend.

### Large File Upload Timeout

```toml
# Increase TUS max file size
[component.etusupload]
maxSize = 5368709120  # 5GB

# The Gateway proxy also needs timeout adjustment
[server.http]
readTimeout = "300s"
writeTimeout = "300s"
```

### Disk Space Exhaustion

TUS temporary files are stored in `dataDir` and only moved to `uploadDir` after upload completes. Ensure both directories have sufficient space. Recommendations:

- `dataDir` should have at least: max concurrent uploads x max file size
- Periodically clean up incomplete TUS uploads (TUS does not auto-clean abandoned uploads)

## Reference Links

- [MinIO Official Documentation](https://min.io/docs/)
- [TUS Protocol Specification](https://tus.io/protocols/resumable-upload)
- [tusd Server Implementation](https://github.com/tus/tusd)
- [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/)
- EgoAdmin source: `internal/platform/objectstore/`
- EgoAdmin source: `internal/component/etusupload/`
