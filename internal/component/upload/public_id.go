package upload

import (
	"fmt"
	"math"

	"github.com/egoadmin/egoadmin/internal/component/idgen/idcodec"
)

const (
	publicFileIDPrefix      = "file"
	publicReferenceIDPrefix = "ref"
)

func PublicFileIDPrefix() string {
	return publicFileIDPrefix
}

func PublicReferenceIDPrefix() string {
	return publicReferenceIDPrefix
}

func (c *Component) encodePublicID(prefix string, id uint64) string {
	out, err := c.publicID(prefix, id)
	if err != nil {
		return ""
	}
	return out
}

func (c *Component) publicID(prefix string, id uint64) (string, error) {
	if id == 0 {
		return "", fmt.Errorf("upload: public id requires a positive id")
	}
	if id > math.MaxInt64 {
		return "", fmt.Errorf("upload: public id exceeds int64 range")
	}
	if c.codec == nil {
		return "", fmt.Errorf("upload: id codec is required")
	}
	out, err := c.codec.Encode(prefix, int64(id))
	if err != nil {
		return "", fmt.Errorf("upload: encode public id: %w", err)
	}
	return out, nil
}

func DecodeReferenceID(codec idcodec.Interface, value string) (uint64, error) {
	if codec == nil {
		return 0, fmt.Errorf("upload: id codec is required")
	}
	id, err := codec.DecodeWithPrefix(publicReferenceIDPrefix, value)
	if err != nil {
		return 0, fmt.Errorf("upload: invalid public reference id: %w", err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("upload: invalid public reference id")
	}
	return uint64(id), nil
}
