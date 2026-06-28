package idcodec

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gotomicro/ego/core/elog"
)

const (
	feistelRounds = 6
)

// Component encodes internal numeric IDs into reversible public IDs.
type Component struct {
	name        string
	config      *Config
	logger      *elog.Component
	alphabet    string
	decodeTable [256]int
}

func newComponent(name string, config *Config, logger *elog.Component) (*Component, error) {
	if config == nil {
		return nil, fmt.Errorf("%w: config is nil", ErrInvalidConfig)
	}
	cp := *config
	cp.normalize()
	if err := cp.validate(); err != nil {
		return nil, err
	}
	alphabet, err := resolveAlphabet(cp.Alphabet)
	if err != nil {
		return nil, err
	}

	comp := &Component{
		name:     name,
		config:   &cp,
		logger:   logger,
		alphabet: alphabet,
	}
	for i := range comp.decodeTable {
		comp.decodeTable[i] = -1
	}
	for i := 0; i < len(alphabet); i++ {
		comp.decodeTable[alphabet[i]] = i
	}
	return comp, nil
}

func (c *Component) Name() string {
	return c.name
}

func (c *Component) PackageName() string {
	return PackageName
}

func (c *Component) Init() error {
	if c == nil || c.config == nil {
		return fmt.Errorf("%w: component is nil", ErrInvalidConfig)
	}
	c.config.normalize()
	return c.config.validate()
}

func (c *Component) Start() error {
	return nil
}

func (c *Component) Stop() error {
	return nil
}

func (c *Component) Close() error {
	return c.Stop()
}

// Encode converts an internal positive int64 ID to a public ID.
func (c *Component) Encode(prefix string, id int64) (out string, err error) {
	begin := time.Now()
	defer func() { c.observe("encode", begin, err) }()

	if err = validatePrefix(prefix, c.config.Separator); err != nil {
		return "", err
	}
	if id <= 0 {
		return "", fmt.Errorf("%w: id must be positive", ErrInvalidID)
	}
	obfuscated := c.permuteInt63(prefix, uint64(id))
	body := c.encodeBase62(obfuscated)
	if len(body) < c.config.MinLength {
		body = strings.Repeat(string(c.alphabet[0]), c.config.MinLength-len(body)) + body
	}
	return prefix + c.config.Separator + body, nil
}

// Decode parses a public ID and returns its prefix and internal numeric ID.
func (c *Component) Decode(value string) (prefix string, id int64, err error) {
	begin := time.Now()
	defer func() { c.observe("decode", begin, err) }()

	prefix, body, err := c.split(value)
	if err != nil {
		return "", 0, err
	}
	id, err = c.decodeBody(prefix, body)
	if err != nil {
		return "", 0, err
	}
	return prefix, id, nil
}

// DecodeWithPrefix parses a public ID and verifies its expected prefix.
func (c *Component) DecodeWithPrefix(prefix string, value string) (id int64, err error) {
	begin := time.Now()
	defer func() { c.observe("decode_with_prefix", begin, err) }()

	if err = validatePrefix(prefix, c.config.Separator); err != nil {
		return 0, err
	}
	gotPrefix, body, err := c.split(value)
	if err != nil {
		return 0, err
	}
	if gotPrefix != prefix {
		return 0, fmt.Errorf("%w: got %q, want %q", ErrInvalidPrefix, gotPrefix, prefix)
	}
	return c.decodeBody(prefix, body)
}

func (c *Component) split(value string) (string, string, error) {
	if value == "" {
		return "", "", fmt.Errorf("%w: value is empty", ErrInvalidFormat)
	}
	idx := strings.LastIndex(value, c.config.Separator)
	if idx <= 0 || idx+len(c.config.Separator) >= len(value) {
		return "", "", fmt.Errorf("%w: missing separator", ErrInvalidFormat)
	}
	prefix := value[:idx]
	body := value[idx+len(c.config.Separator):]
	if err := validatePrefix(prefix, c.config.Separator); err != nil {
		return "", "", err
	}
	if body == "" {
		return "", "", fmt.Errorf("%w: body is empty", ErrInvalidFormat)
	}
	return prefix, body, nil
}

func (c *Component) decodeBody(prefix string, body string) (int64, error) {
	obfuscated, err := c.decodeBase62(body)
	if err != nil {
		return 0, err
	}
	id := c.unpermuteInt63(prefix, obfuscated)
	if id == 0 || id > math.MaxInt64 {
		return 0, fmt.Errorf("%w: decoded id is out of int64 range", ErrOverflow)
	}
	return int64(id), nil
}

func validatePrefix(prefix string, separator string) error {
	if prefix == "" {
		return fmt.Errorf("%w: prefix is empty", ErrInvalidPrefix)
	}
	if strings.Contains(prefix, separator) {
		return fmt.Errorf("%w: prefix contains separator", ErrInvalidPrefix)
	}
	for i := 0; i < len(prefix); i++ {
		ch := prefix[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			continue
		}
		return fmt.Errorf("%w: prefix contains invalid character %q", ErrInvalidPrefix, ch)
	}
	return nil
}

func (c *Component) encodeBase62(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [11]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = c.alphabet[v%62]
		v /= 62
	}
	return string(buf[i:])
}

func (c *Component) decodeBase62(value string) (uint64, error) {
	var out uint64
	for i := 0; i < len(value); i++ {
		ch := value[i]
		digit := -1
		if int(ch) < len(c.decodeTable) {
			digit = c.decodeTable[ch]
		}
		if digit < 0 {
			return 0, fmt.Errorf("%w: invalid base62 character %q", ErrInvalidFormat, ch)
		}
		if out > (math.MaxUint64-uint64(digit))/62 {
			return 0, fmt.Errorf("%w: base62 value overflow", ErrOverflow)
		}
		out = out*62 + uint64(digit)
	}
	return out, nil
}

func (c *Component) permute(prefix string, value uint64) uint64 {
	left, right := splitUint64(value)
	for round := 0; round < feistelRounds; round++ {
		nextLeft := right
		nextRight := left ^ c.round(prefix, round, right)
		left, right = nextLeft, nextRight
	}
	return uint64(left)<<32 | uint64(right)
}

func (c *Component) unpermute(prefix string, value uint64) uint64 {
	left, right := splitUint64(value)
	for round := feistelRounds - 1; round >= 0; round-- {
		prevRight := left
		prevLeft := right ^ c.round(prefix, round, prevRight)
		left, right = prevLeft, prevRight
	}
	return uint64(left)<<32 | uint64(right)
}

func splitUint64(value uint64) (uint32, uint32) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], value)
	left := binary.BigEndian.Uint32(buf[:4])
	right := binary.BigEndian.Uint32(buf[4:])
	return left, right
}

func (c *Component) permuteInt63(prefix string, value uint64) uint64 {
	for {
		next := c.permute(prefix, value)
		if next <= math.MaxInt64 {
			return next
		}
		value = next
	}
}

func (c *Component) unpermuteInt63(prefix string, value uint64) uint64 {
	for {
		next := c.unpermute(prefix, value)
		if next <= math.MaxInt64 {
			return next
		}
		value = next
	}
}

func (c *Component) round(prefix string, round int, right uint32) uint32 {
	mac := hmac.New(sha256.New, []byte(c.config.Secret))
	_, _ = mac.Write([]byte(prefix))
	_, _ = mac.Write([]byte{0})
	var buf [5]byte
	buf[0] = byte(round)
	binary.BigEndian.PutUint32(buf[1:], right)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)
	return binary.BigEndian.Uint32(sum[4:8])
}
