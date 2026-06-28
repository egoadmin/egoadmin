package idcodec

import (
	"errors"
	"math"
	"strings"
	"testing"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func newTestCodec(t *testing.T) *Component {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Secret = testSecret
	cfg.EnableMetrics = false
	return DefaultContainer().Build(WithConfig(cfg), WithName("component.idgen.codec.test"))
}

func TestEncodeDecode(t *testing.T) {
	codec := newTestCodec(t)

	publicID, err := codec.Encode("order", 2)
	if err != nil {
		t.Fatal(err)
	}
	if publicID != "order-07uQlcBmL6d0" {
		t.Fatalf("publicID = %q, want stable encoded value", publicID)
	}

	prefix, id, err := codec.Decode(publicID)
	if err != nil {
		t.Fatal(err)
	}
	if prefix != "order" || id != 2 {
		t.Fatalf("Decode() = %q, %d, want order, 2", prefix, id)
	}

	id, err = codec.DecodeWithPrefix("order", publicID)
	if err != nil {
		t.Fatal(err)
	}
	if id != 2 {
		t.Fatalf("DecodeWithPrefix() = %d, want 2", id)
	}
}

func TestEncodeUsesPrefixInKeyDerivation(t *testing.T) {
	codec := newTestCodec(t)

	orderID, err := codec.Encode("order", 2)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := codec.Encode("user", 2)
	if err != nil {
		t.Fatal(err)
	}
	if orderID == userID {
		t.Fatalf("same numeric ID encoded to same public ID across prefixes: %q", orderID)
	}
}

func TestCustomAlphabetRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Secret = testSecret
	cfg.Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	cfg.MinLength = 18
	cfg.EnableMetrics = false
	codec := DefaultContainer().Build(WithConfig(cfg), WithName("component.idgen.codec.test"))

	publicID, err := codec.Encode("order", 2)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.TrimPrefix(publicID, "order-")
	if !strings.HasPrefix(body, "A") {
		t.Fatalf("body = %q, want custom alphabet padding with A", body)
	}
	id, err := codec.DecodeWithPrefix("order", publicID)
	if err != nil {
		t.Fatal(err)
	}
	if id != 2 {
		t.Fatalf("id = %d, want 2", id)
	}
}

func TestDecodeWithPrefixRejectsWrongPrefix(t *testing.T) {
	codec := newTestCodec(t)
	publicID, err := codec.Encode("order", 2)
	if err != nil {
		t.Fatal(err)
	}

	_, err = codec.DecodeWithPrefix("user", publicID)
	if !errors.Is(err, ErrInvalidPrefix) {
		t.Fatalf("DecodeWithPrefix() error = %v, want ErrInvalidPrefix", err)
	}
}

func TestDecodeRejectsInvalidFormat(t *testing.T) {
	codec := newTestCodec(t)
	tests := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "missing separator", value: "order"},
		{name: "empty body", value: "order-"},
		{name: "invalid character", value: "order-abc!"},
		{name: "invalid prefix", value: "bad/prefix-abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := codec.Decode(tt.value)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestEncodeRejectsInvalidInput(t *testing.T) {
	codec := newTestCodec(t)
	tests := []struct {
		name   string
		prefix string
		id     int64
		want   error
	}{
		{name: "empty prefix", prefix: "", id: 1, want: ErrInvalidPrefix},
		{name: "prefix contains separator", prefix: "bad-prefix", id: 1, want: ErrInvalidPrefix},
		{name: "zero id", prefix: "order", id: 0, want: ErrInvalidID},
		{name: "negative id", prefix: "order", id: -1, want: ErrInvalidID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := codec.Encode(tt.prefix, tt.id)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Encode() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestMaxInt64RoundTrip(t *testing.T) {
	codec := newTestCodec(t)
	publicID, err := codec.Encode("order", math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	id, err := codec.DecodeWithPrefix("order", publicID)
	if err != nil {
		t.Fatal(err)
	}
	if id != math.MaxInt64 {
		t.Fatalf("id = %d, want MaxInt64", id)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "empty secret",
			mutate: func(c *Config) {
				c.Secret = ""
			},
		},
		{
			name: "short secret",
			mutate: func(c *Config) {
				c.Secret = "short"
			},
		},
		{
			name: "unsupported algorithm",
			mutate: func(c *Config) {
				c.Secret = testSecret
				c.Algorithm = "unknown"
			},
		},
		{
			name: "duplicate alphabet",
			mutate: func(c *Config) {
				c.Secret = testSecret
				c.Alphabet = strings.Repeat("a", 62)
			},
		},
		{
			name: "separator conflicts with alphabet",
			mutate: func(c *Config) {
				c.Secret = testSecret
				c.Separator = "a"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(cfg)
			cfg.normalize()
			err := cfg.validate()
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("validate() error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}
