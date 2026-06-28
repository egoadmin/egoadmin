package cdn

import (
	"net/url"
	"strings"
	"time"

	"github.com/gotomicro/ego/core/util/xtime"
)

const PackageName = "component.cdn"

type Config struct {
	FilePath            string        `toml:"filePath" json:"filePath"`
	ImagePath           string        `toml:"imagePath" json:"imagePath"`
	SignSecret          string        `toml:"signSecret" json:"signSecret"`
	DefaultSignedTTL    time.Duration `toml:"defaultSignedTTL" json:"defaultSignedTTL"`
	PublicImage         bool          `toml:"publicImage" json:"publicImage"`
	AllowTemporaryImage bool          `toml:"allowTemporaryImage" json:"allowTemporaryImage"`
	MaxProcessPathBytes int           `toml:"maxProcessPathBytes" json:"maxProcessPathBytes"`
	MaxQueryBytes       int           `toml:"maxQueryBytes" json:"maxQueryBytes"`
}

type ImageProcessorConfig struct {
	URL     string        `toml:"url" json:"url"`
	Secret  string        `toml:"secret" json:"secret"`
	Timeout time.Duration `toml:"timeout" json:"timeout"`
}

func DefaultConfig() *Config {
	return &Config{
		FilePath:            "/cdn/file",
		ImagePath:           "/cdn/image",
		SignSecret:          "change-me",
		DefaultSignedTTL:    xtime.Duration("10m"),
		PublicImage:         true,
		AllowTemporaryImage: false,
		MaxProcessPathBytes: 2048,
		MaxQueryBytes:       2048,
	}
}

func DefaultImageProcessorConfig() *ImageProcessorConfig {
	return &ImageProcessorConfig{
		URL:     "http://image-processor:2853",
		Secret:  "change-me",
		Timeout: xtime.Duration("5s"),
	}
}

func (c *Config) Normalize() error {
	defaults := DefaultConfig()
	if c.FilePath == "" {
		c.FilePath = defaults.FilePath
	}
	if c.ImagePath == "" {
		c.ImagePath = defaults.ImagePath
	}
	if c.SignSecret == "" {
		c.SignSecret = defaults.SignSecret
	}
	if c.DefaultSignedTTL <= 0 {
		c.DefaultSignedTTL = defaults.DefaultSignedTTL
	}
	if c.MaxProcessPathBytes <= 0 {
		c.MaxProcessPathBytes = defaults.MaxProcessPathBytes
	}
	if c.MaxQueryBytes <= 0 {
		c.MaxQueryBytes = defaults.MaxQueryBytes
	}
	c.FilePath = cleanRoutePath(c.FilePath)
	c.ImagePath = cleanRoutePath(c.ImagePath)
	return nil
}

func (c *ImageProcessorConfig) Normalize() error {
	defaults := DefaultImageProcessorConfig()
	if c.URL == "" {
		c.URL = defaults.URL
	}
	if c.Secret == "" {
		c.Secret = defaults.Secret
	}
	if c.Timeout <= 0 {
		c.Timeout = defaults.Timeout
	}
	c.URL = strings.TrimRight(c.URL, "/")
	return nil
}

func cleanRoutePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	u, err := url.Parse(value)
	if err == nil && u.Path != "" {
		value = u.Path
	}
	value = "/" + strings.Trim(value, "/")
	if value == "/" {
		return ""
	}
	return value
}
