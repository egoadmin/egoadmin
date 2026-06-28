package idcodec

import (
	"fmt"
	"strings"
)

const (
	// PackageName identifies this component in EGO logs and metrics.
	PackageName = "component.idgen.codec"

	defaultComponentName = "component.idgen.codec"
	defaultAlphabet      = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	AlgorithmFeistelBase62 = "feistel-base62"
)

// Config controls public ID encoding and decoding.
type Config struct {
	Secret        string `json:"secret" toml:"secret"`
	Algorithm     string `json:"algorithm" toml:"algorithm"`
	Alphabet      string `json:"alphabet" toml:"alphabet"`
	MinLength     int    `json:"minLength" toml:"minLength"`
	Separator     string `json:"separator" toml:"separator"`
	EnableMetrics bool   `json:"enableMetrics" toml:"enableMetrics"`
}

// DefaultConfig returns non-sensitive defaults. Secret must be supplied by
// runtime config or options when the component is built.
func DefaultConfig() *Config {
	return &Config{
		Algorithm:     AlgorithmFeistelBase62,
		Alphabet:      "base62",
		MinLength:     12,
		Separator:     "-",
		EnableMetrics: true,
	}
}

func (c *Config) normalize() {
	defaults := DefaultConfig()
	if c.Algorithm == "" {
		c.Algorithm = defaults.Algorithm
	}
	if c.Alphabet == "" {
		c.Alphabet = defaults.Alphabet
	}
	if c.MinLength <= 0 {
		c.MinLength = defaults.MinLength
	}
	if c.Separator == "" {
		c.Separator = defaults.Separator
	}
}

func (c *Config) validate() error {
	if c == nil {
		return fmt.Errorf("%w: config is nil", ErrInvalidConfig)
	}
	if c.Secret == "" {
		return fmt.Errorf("%w: secret is empty", ErrInvalidConfig)
	}
	if len(c.Secret) < 16 {
		return fmt.Errorf("%w: secret must be at least 16 bytes", ErrInvalidConfig)
	}
	if c.Algorithm != AlgorithmFeistelBase62 {
		return fmt.Errorf("%w: unsupported algorithm %q", ErrInvalidConfig, c.Algorithm)
	}
	if c.MinLength <= 0 || c.MinLength > 64 {
		return fmt.Errorf("%w: minLength must be between 1 and 64", ErrInvalidConfig)
	}
	if c.Separator == "" {
		return fmt.Errorf("%w: separator is empty", ErrInvalidConfig)
	}
	alphabet, err := resolveAlphabet(c.Alphabet)
	if err != nil {
		return err
	}
	if strings.ContainsAny(c.Separator, alphabet) {
		return fmt.Errorf("%w: separator must not contain alphabet characters", ErrInvalidConfig)
	}
	return nil
}

func resolveAlphabet(value string) (string, error) {
	if value == "" || value == "base62" {
		return defaultAlphabet, nil
	}
	if len(value) != 62 {
		return "", fmt.Errorf("%w: alphabet must contain 62 ASCII characters", ErrInvalidConfig)
	}
	seen := make(map[byte]struct{}, len(value))
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch > 127 {
			return "", fmt.Errorf("%w: alphabet must be ASCII", ErrInvalidConfig)
		}
		if _, ok := seen[ch]; ok {
			return "", fmt.Errorf("%w: alphabet contains duplicate character %q", ErrInvalidConfig, ch)
		}
		seen[ch] = struct{}{}
	}
	return value, nil
}
