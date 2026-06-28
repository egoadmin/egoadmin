package cdn

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/upload"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server/egin"
)

var ProviderSet = wire.NewSet(NewComponentProvider)

type AuthContext struct {
	UserID uint64
}

type Options struct {
	BeforeFileHandle func(*gin.Context) (*AuthContext, error)
}

type Component struct {
	config         *Config
	imageProcessor *ImageProcessorConfig
	upload         *upload.Component
	object         upload.ObjectStore
	client         *http.Client
}

type Option func(*Component)

func Load(key string, uploadComponent *upload.Component, object upload.ObjectStore, opts ...Option) (*Component, error) {
	config := DefaultConfig()
	if err := econf.UnmarshalKey(key, config); err != nil {
		elog.Warn("unmarshal cdn config failed", elog.String("key", key), elog.FieldErr(err))
	}
	imageProcessor := DefaultImageProcessorConfig()
	if err := econf.UnmarshalKey("client.imageProcessor", imageProcessor); err != nil {
		elog.Warn("unmarshal image processor config failed", elog.FieldErr(err))
	}
	opts = append([]Option{WithConfig(config), WithImageProcessorConfig(imageProcessor)}, opts...)
	return New(uploadComponent, object, opts...)
}

func NewComponentProvider(uploadComponent *upload.Component, object upload.ObjectStore) (*Component, error) {
	return Load(PackageName, uploadComponent, object)
}

func New(uploadComponent *upload.Component, object upload.ObjectStore, opts ...Option) (*Component, error) {
	component := &Component{
		config:         DefaultConfig(),
		imageProcessor: DefaultImageProcessorConfig(),
		upload:         uploadComponent,
		object:         object,
	}
	for _, opt := range opts {
		opt(component)
	}
	if err := component.config.Normalize(); err != nil {
		return nil, err
	}
	if err := component.imageProcessor.Normalize(); err != nil {
		return nil, err
	}
	if component.upload == nil {
		return nil, fmt.Errorf("cdn: upload component is required")
	}
	if component.object == nil {
		return nil, fmt.Errorf("cdn: object store is required")
	}
	if component.client == nil {
		component.client = &http.Client{Timeout: component.imageProcessor.Timeout}
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

func WithImageProcessorConfig(config *ImageProcessorConfig) Option {
	return func(component *Component) {
		if config != nil {
			component.imageProcessor = config
		}
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(component *Component) {
		if client != nil {
			component.client = client
		}
	}
}

func (c *Component) Config() *Config {
	return c.config
}

func (c *Component) RegisterRoutes(server *egin.Component, opts Options) {
	server.GET(c.config.FilePath+"/:referenceId", c.FileHandler(opts))
	server.GET(c.config.ImagePath+"/:referenceId", c.ImageHandler())
	server.GET(c.config.ImagePath+"/:referenceId/*processPath", c.ImageHandler())
}

func RegisterRoutes(server *egin.Component, component *Component, opts Options) {
	component.RegisterRoutes(server, opts)
}

func (c *Component) SignedFileURL(referenceID uint64, display string, now time.Time) string {
	expiresAt := now.Add(c.config.DefaultSignedTTL)
	publicReferenceID := c.publicReferenceID(referenceID)
	if publicReferenceID == "" {
		return ""
	}
	path := fmt.Sprintf("%s/%s", c.config.FilePath, publicReferenceID)
	if display != "" {
		values := url.Values{}
		values.Set("display", display)
		path += "?" + values.Encode()
	}
	return signedURL(path, c.config.SignSecret, expiresAt)
}

func (c *Component) SignedImageURL(referenceID uint64, processPath string, now time.Time) string {
	expiresAt := now.Add(c.config.DefaultSignedTTL)
	publicReferenceID := c.publicReferenceID(referenceID)
	if publicReferenceID == "" {
		return ""
	}
	path := fmt.Sprintf("%s/%s", c.config.ImagePath, publicReferenceID)
	processPath = strings.Trim(processPath, "/")
	if processPath != "" {
		path += "/" + processPath
	}
	return signedURL(path, c.config.SignSecret, expiresAt)
}

func (c *Component) parseReferenceID(raw string) (uint64, error) {
	if id, err := upload.DecodeReferenceID(c.upload.IDCodec(), raw); err == nil {
		return id, nil
	}
	return 0, ErrInvalidReferenceID
}

func (c *Component) publicReferenceID(referenceID uint64) string {
	if publicID := c.upload.PublicReferenceID(referenceID); publicID != "" {
		return publicID
	}
	return ""
}
