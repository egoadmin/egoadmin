package objectstore

import (
	"context"
	"errors"
	"io"
	"strings"

	uploadcomponent "github.com/egoadmin/egoadmin/internal/component/upload"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/egoadmin/internal/platform/defaults"
	"github.com/egoadmin/elib/pkg/util/xfile"
	"github.com/egoadmin/eminio"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/wire"
	"github.com/gotomicro/ego/core/econf"
	"github.com/minio/minio-go/v7"
	tuss3store "github.com/tus/tusd/v2/pkg/s3store"
)

var ProviderSet = wire.NewSet(
	NewEMinio, NewS3, NewUploadObjectStore, NewTusS3API,
	wire.Bind(new(uploadcomponent.ObjectStore), new(*UploadObjectStore)),
	wire.Bind(new(uploadcomponent.TusS3API), new(tuss3store.S3API)),
)

type minioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Ssl             bool
	Region          string
}

// NewEMinio 初始化minio
func NewEMinio() *eminio.Component {
	return eminio.Load("client.minio").Build()
}

// NewS3 初始化上传库
func NewS3(com *eminio.Component, conf *config.Config) *xfile.S3 {
	bucketName := conf.App().BucketName
	if bucketName == "" {
		bucketName = defaults.MinioBucketName
	}
	return xfile.NewS3(com,
		xfile.WithS3AutoCreateBucket(),
		xfile.WithS3BucketName(bucketName))
}

type UploadObjectStore struct {
	client *eminio.Component
	bucket string
}

type objectReader struct {
	object *minio.Object
	key    string
}

func NewUploadObjectStore(com *eminio.Component, conf *config.Config) *UploadObjectStore {
	bucketName := conf.App().BucketName
	if bucketName == "" {
		bucketName = defaults.MinioBucketName
	}
	return &UploadObjectStore{
		client: com,
		bucket: bucketName,
	}
}

func NewTusS3API() tuss3store.S3API {
	cfg := minioConfig{
		Endpoint: "localhost:9000",
		Region:   "us-east-1",
	}
	_ = econf.UnmarshalKey("client.minio", &cfg)
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	endpoint := cfg.Endpoint
	if endpoint != "" && !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if cfg.Ssl {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}
	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
	}
	return s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
			options.UsePathStyle = true
		}
		options.EndpointOptions.DisableHTTPS = !cfg.Ssl
	})
}

func (s *UploadObjectStore) Put(ctx context.Context, key string, reader io.Reader, size int64, opts uploadcomponent.PutOptions) error {
	_, err := s.client.Client().PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: opts.ContentType,
	})
	return err
}

func (s *UploadObjectStore) Get(ctx context.Context, key string) (uploadcomponent.ObjectReader, error) {
	object, err := s.client.Client().GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		if isMinioNotFound(err) {
			return nil, uploadcomponent.ErrObjectNotFound
		}
		return nil, err
	}
	return &objectReader{object: object, key: key}, nil
}

func (s *UploadObjectStore) Delete(ctx context.Context, key string) error {
	err := s.client.Client().RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if isMinioNotFound(err) {
		return uploadcomponent.ErrObjectNotFound
	}
	return err
}

func (s *UploadObjectStore) Stat(ctx context.Context, key string) (uploadcomponent.ObjectInfo, error) {
	info, err := s.client.Client().StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isMinioNotFound(err) {
			return uploadcomponent.ObjectInfo{}, uploadcomponent.ErrObjectNotFound
		}
		return uploadcomponent.ObjectInfo{}, err
	}
	return uploadcomponent.ObjectInfo{
		Key:         key,
		Size:        info.Size,
		ContentType: info.ContentType,
	}, nil
}

func (r *objectReader) Read(p []byte) (int, error) {
	return r.object.Read(p)
}

func (r *objectReader) Seek(offset int64, whence int) (int64, error) {
	return r.object.Seek(offset, whence)
}

func (r *objectReader) Close() error {
	return r.object.Close()
}

func (r *objectReader) Stat() (uploadcomponent.ObjectInfo, error) {
	info, err := r.object.Stat()
	if err != nil {
		if isMinioNotFound(err) {
			return uploadcomponent.ObjectInfo{}, uploadcomponent.ErrObjectNotFound
		}
		return uploadcomponent.ObjectInfo{}, err
	}
	return uploadcomponent.ObjectInfo{
		Key:         r.key,
		Size:        info.Size,
		ContentType: info.ContentType,
	}, nil
}

func isMinioNotFound(err error) bool {
	if err == nil {
		return false
	}
	var response minio.ErrorResponse
	if errors.As(err, &response) {
		return response.Code == "NoSuchKey"
	}
	return false
}
