# etusupload quickstart

`etusupload` provides optional TUS resumable upload support. It is not wired into the gateway server by default and does not replace the existing S3/MinIO `/upload` endpoint.

## Config

```toml
[component.etusupload]
basePath = "/tus/upload"
maxSize = 1073741824
dataDir = "./data/tus"
uploadDir = "./uploads"
enableValidation = true
validateBeforeUpload = true
validateAfterUpload = true
allowAllOrigins = true
```

## Register Routes

```go
import "github.com/egoadmin/egoadmin/internal/component/etusupload"

func registerTus(engine *gin.Engine) error {
	comp := etusupload.Load("component.etusupload").Build()
	engine.Use(etusupload.NewCorsMiddleware(comp.GetConfig()))
	return comp.RegisterRoutes(engine)
}
```

The default endpoint is `/tus/upload`. Use a different path only when it does not conflict with the existing multipart upload route.

## Hooks

Register an `UploadHook` to persist metadata or copy completed local files to S3/MinIO.

```go
type UploadHook struct{}

func (h *UploadHook) OnBeforeUpload(ctx context.Context, filename string) error {
	return nil
}

func (h *UploadHook) OnAfterUpload(ctx context.Context, info *etusupload.UploadInfo) error {
	// Store info.Metadata or transfer info.FilePath to object storage here.
	return nil
}

func (h *UploadHook) OnValidationFailed(ctx context.Context, filename string, err error) error {
	return nil
}
```

## Client Example

```js
const upload = new tus.Upload(file, {
  endpoint: '/tus/upload',
  metadata: { filename: file.name },
})
upload.start()
```
