package upload

import (
	"fmt"
	"strings"
	"time"

	"github.com/gotomicro/ego/core/util/xtime"
)

const (
	defaultMaxSize              = int64(100 * 1024 * 1024)
	defaultTTL                  = 24 * time.Hour
	defaultPartSize             = int64(32 * 1024 * 1024)
	defaultMaxBufferedParts     = int64(4)
	defaultMaxConcurrentUploads = 8
	defaultMaxTempDirSize       = int64(2 * 1024 * 1024 * 1024)
)

type Config struct {
	MultipartPath string                   `toml:"multipartPath" json:"multipartPath" default:"/upload"`
	DefaultTTL    time.Duration            `toml:"defaultTTL" json:"defaultTTL" default:"24h"`
	Profiles      map[string]ProfileConfig `toml:"profiles" json:"profiles"`
	Tus           TusConfig                `toml:"tus" json:"tus"`
}

type TusConfig struct {
	Enabled              bool          `toml:"enabled" json:"enabled"`
	Path                 string        `toml:"path" json:"path"`
	TemporaryDirectory   string        `toml:"temporaryDirectory" json:"temporaryDirectory"`
	ObjectPrefix         string        `toml:"objectPrefix" json:"objectPrefix"`
	MetadataPrefix       string        `toml:"metadataPrefix" json:"metadataPrefix"`
	PartSize             int64         `toml:"partSize" json:"partSize"`
	MaxBufferedParts     int64         `toml:"maxBufferedParts" json:"maxBufferedParts"`
	MaxConcurrentUploads int           `toml:"maxConcurrentUploads" json:"maxConcurrentUploads"`
	MaxTempDirSize       int64         `toml:"maxTempDirSize" json:"maxTempDirSize"`
	LocalTempTTL         time.Duration `toml:"localTempTTL" json:"localTempTTL"`
}

type ProfileConfig struct {
	Extends           string        `toml:"extends" json:"extends"`
	MaxSize           int64         `toml:"maxSize" json:"maxSize"`
	TTL               time.Duration `toml:"ttl" json:"ttl"`
	AllowedExtensions []string      `toml:"allowedExtensions" json:"allowedExtensions"`
	AllowedMimeTypes  []string      `toml:"allowedMimeTypes" json:"allowedMimeTypes"`
	TusRequired       bool          `toml:"tusRequired" json:"tusRequired"`
	MaxCount          int           `toml:"maxCount" json:"maxCount"`
}

func DefaultConfig() *Config {
	return &Config{
		MultipartPath: "/upload",
		DefaultTTL:    xtime.Duration("24h"),
		Profiles: map[string]ProfileConfig{
			DefaultProfile: {
				MaxSize: defaultMaxSize,
				TTL:     xtime.Duration("24h"),
			},
			"image": {
				MaxSize:           10 * 1024 * 1024,
				TTL:               xtime.Duration("24h"),
				AllowedExtensions: []string{"jpg", "jpeg", "png", "webp", "gif"},
				AllowedMimeTypes:  []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
			},
			"avatar": {
				Extends:  "image",
				MaxSize:  5 * 1024 * 1024,
				MaxCount: 1,
			},
			"document": {
				MaxSize:           50 * 1024 * 1024,
				TTL:               xtime.Duration("24h"),
				AllowedExtensions: []string{"pdf", "doc", "docx", "xls", "xlsx", "txt"},
			},
			"video": {
				MaxSize:     2 * 1024 * 1024 * 1024,
				TTL:         xtime.Duration("24h"),
				TusRequired: true,
			},
		},
		Tus: TusConfig{
			Enabled:              false,
			Path:                 "/tus/upload",
			TemporaryDirectory:   "./data/tus-tmp",
			ObjectPrefix:         "files",
			MetadataPrefix:       "tus-meta",
			PartSize:             defaultPartSize,
			MaxBufferedParts:     defaultMaxBufferedParts,
			MaxConcurrentUploads: defaultMaxConcurrentUploads,
			MaxTempDirSize:       defaultMaxTempDirSize,
			LocalTempTTL:         xtime.Duration("24h"),
		},
	}
}

func (c *Config) Normalize() error {
	if c.MultipartPath == "" {
		c.MultipartPath = "/upload"
	}
	if c.DefaultTTL <= 0 {
		c.DefaultTTL = defaultTTL
	}
	if c.Profiles == nil {
		c.Profiles = map[string]ProfileConfig{}
	}
	c.normalizeTus()
	defaults := DefaultConfig()
	for name, profile := range defaults.Profiles {
		if _, ok := c.Profiles[name]; !ok {
			c.Profiles[name] = profile
		}
	}
	resolved := make(map[string]ProfileConfig, len(c.Profiles))
	for name := range c.Profiles {
		profile, err := c.resolveProfile(name, map[string]bool{})
		if err != nil {
			return err
		}
		if profile.MaxSize <= 0 {
			profile.MaxSize = defaultMaxSize
		}
		if profile.TTL <= 0 {
			profile.TTL = c.DefaultTTL
		}
		profile.AllowedExtensions = normalizeList(profile.AllowedExtensions)
		profile.AllowedMimeTypes = normalizeList(profile.AllowedMimeTypes)
		resolved[name] = profile
	}
	c.Profiles = resolved
	return nil
}

func (c *Config) normalizeTus() {
	defaults := DefaultConfig().Tus
	if c.Tus.Path == "" {
		c.Tus.Path = defaults.Path
	}
	if c.Tus.TemporaryDirectory == "" {
		c.Tus.TemporaryDirectory = defaults.TemporaryDirectory
	}
	if c.Tus.ObjectPrefix == "" {
		c.Tus.ObjectPrefix = defaults.ObjectPrefix
	}
	if c.Tus.MetadataPrefix == "" {
		c.Tus.MetadataPrefix = defaults.MetadataPrefix
	}
	c.Tus.ObjectPrefix = strings.Trim(c.Tus.ObjectPrefix, "/")
	c.Tus.MetadataPrefix = strings.Trim(c.Tus.MetadataPrefix, "/")
	if c.Tus.PartSize <= 0 {
		c.Tus.PartSize = defaults.PartSize
	}
	if c.Tus.MaxBufferedParts <= 0 {
		c.Tus.MaxBufferedParts = defaults.MaxBufferedParts
	}
	if c.Tus.MaxConcurrentUploads <= 0 {
		c.Tus.MaxConcurrentUploads = defaults.MaxConcurrentUploads
	}
	if c.Tus.MaxTempDirSize <= 0 {
		c.Tus.MaxTempDirSize = defaults.MaxTempDirSize
	}
	if c.Tus.LocalTempTTL <= 0 {
		c.Tus.LocalTempTTL = defaults.LocalTempTTL
	}
}

func (c *Config) Profile(name string) ProfileConfig {
	if name == "" {
		name = DefaultProfile
	}
	profile, ok := c.Profiles[name]
	if ok {
		return profile
	}
	return c.Profiles[DefaultProfile]
}

func (c *Config) RequireProfile(name string) (string, ProfileConfig, error) {
	if name == "" {
		name = DefaultProfile
	}
	profile, ok := c.Profiles[name]
	if !ok {
		return "", ProfileConfig{}, fmt.Errorf("upload: profile %q not found", name)
	}
	return name, profile, nil
}

func (c *Config) resolveProfile(name string, seen map[string]bool) (ProfileConfig, error) {
	profile, ok := c.Profiles[name]
	if !ok {
		return ProfileConfig{}, fmt.Errorf("upload: profile %q not found", name)
	}
	if profile.Extends == "" {
		return profile, nil
	}
	if seen[name] {
		return ProfileConfig{}, fmt.Errorf("upload: profile %q extends cycle", name)
	}
	seen[name] = true
	base, err := c.resolveProfile(profile.Extends, seen)
	if err != nil {
		return ProfileConfig{}, err
	}
	return mergeProfile(base, profile), nil
}

func mergeProfile(base ProfileConfig, override ProfileConfig) ProfileConfig {
	base.Extends = override.Extends
	if override.MaxSize > 0 {
		base.MaxSize = override.MaxSize
	}
	if override.TTL > 0 {
		base.TTL = override.TTL
	}
	if len(override.AllowedExtensions) > 0 {
		base.AllowedExtensions = override.AllowedExtensions
	}
	if len(override.AllowedMimeTypes) > 0 {
		base.AllowedMimeTypes = override.AllowedMimeTypes
	}
	if override.TusRequired {
		base.TusRequired = true
	}
	if override.MaxCount > 0 {
		base.MaxCount = override.MaxCount
	}
	return base
}

func normalizeList(items []string) []string {
	normalized := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(strings.ToLower(item))
		item = strings.TrimPrefix(item, ".")
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	return normalized
}
