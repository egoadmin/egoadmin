package upload

import (
	"fmt"
	"sync"

	"github.com/egoadmin/egoadmin/internal/component/idgen/idcodec"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/elib/pkg/util/xflake"
	"github.com/google/wire"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
)

const PackageName = "component.upload"

var ProviderSet = wire.NewSet(
	NewComponentProvider,
)

func Load(key string, store MetadataStore, object ObjectStore, flake xflake.Geter, tusS3 TusS3API, opts ...Option) (*Component, error) {
	config := DefaultConfig()
	if err := econf.UnmarshalKey(key, config); err != nil {
		elog.Warn("unmarshal upload config failed", elog.String("key", key), elog.FieldErr(err))
	}
	opts = append([]Option{WithConfig(config)}, opts...)
	return New(store, object, flake, append(opts, WithTusS3API(tusS3))...)
}

func NewComponentProvider(conf *config.Config, store MetadataStore, object ObjectStore, flake xflake.Geter, tusS3 TusS3API, codec *idcodec.Component) (*Component, error) {
	app := conf.App()
	web := conf.Web()
	cdnPaths := defaultAccessPaths()
	if err := econf.UnmarshalKey("component.cdn", &cdnPaths); err != nil {
		elog.Warn("unmarshal cdn paths for upload access url failed", elog.String("key", "component.cdn"), elog.FieldErr(err))
	}
	return Load(PackageName, store, object, flake, tusS3,
		WithBucket(app.BucketName),
		WithBaseURL(web.FileBaseUrl),
		WithAccessPaths(cdnPaths.FilePath, cdnPaths.ImagePath),
		WithIDCodec(codec),
	)
}

type accessPaths struct {
	FilePath  string `toml:"filePath"`
	ImagePath string `toml:"imagePath"`
}

func defaultAccessPaths() accessPaths {
	return accessPaths{
		FilePath:  "/cdn/file",
		ImagePath: "/cdn/image",
	}
}

type Component struct {
	config       *Config
	store        MetadataStore
	object       ObjectStore
	flake        xflake.Geter
	bucket       string
	baseURL      string
	fileURLPath  string
	imageURLPath string
	tusS3        TusS3API
	codec        idcodec.Interface
	tus          *TusServer
	closeMu      sync.Mutex
	closed       bool
}

type Option func(*Component)

func New(store MetadataStore, object ObjectStore, flake xflake.Geter, opts ...Option) (*Component, error) {
	component := &Component{
		config:       DefaultConfig(),
		store:        store,
		object:       object,
		flake:        flake,
		fileURLPath:  "/cdn/file",
		imageURLPath: "/cdn/image",
	}
	for _, opt := range opts {
		opt(component)
	}
	if err := component.config.Normalize(); err != nil {
		return nil, err
	}
	if component.store == nil {
		return nil, fmt.Errorf("upload: metadata store is required")
	}
	if component.object == nil {
		return nil, fmt.Errorf("upload: object store is required")
	}
	if component.flake == nil {
		return nil, fmt.Errorf("upload: id generator is required")
	}
	if component.codec == nil {
		return nil, fmt.Errorf("upload: id codec is required")
	}
	return component, nil
}

func WithConfig(config *Config) Option {
	return func(component *Component) {
		if config != nil {
			component.config = config
		}
	}
}

func WithBucket(bucket string) Option {
	return func(component *Component) {
		component.bucket = bucket
	}
}

func WithBaseURL(baseURL string) Option {
	return func(component *Component) {
		component.baseURL = baseURL
	}
}

func WithAccessPaths(filePath string, imagePath string) Option {
	return func(component *Component) {
		if filePath != "" {
			component.fileURLPath = filePath
		}
		if imagePath != "" {
			component.imageURLPath = imagePath
		}
	}
}

func WithTusS3API(api TusS3API) Option {
	return func(component *Component) {
		component.tusS3 = api
	}
}

func WithIDCodec(codec idcodec.Interface) Option {
	return func(component *Component) {
		component.codec = codec
	}
}

func (c *Component) Config() *Config {
	return c.config
}

func (c *Component) IDCodec() idcodec.Interface {
	return c.codec
}

func (c *Component) PublicReferenceID(referenceID uint64) string {
	return c.encodePublicID(publicReferenceIDPrefix, referenceID)
}

func (c *Component) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.tus != nil {
		return c.tus.Close()
	}
	return nil
}
