package captcha

import (
	"strings"

	b64captcha "github.com/mojocn/base64Captcha"
)

// newMemoryStore 新建验证码内存存储.
func newMemoryStore() *MyCaptchaStore {
	return &MyCaptchaStore{
		store: b64captcha.DefaultMemStore,
	}
}

// b64captcha.DefaultMemStore的类型为---var b64captcha.DefaultMemStore b64captcha.Store
// MyCaptchaStore 验证码内存存储.
type MyCaptchaStore struct {
	store b64captcha.Store
}

// Set sets the digits for the captcha id.
func (s *MyCaptchaStore) Set(id string, value string) error {
	return s.store.Set(id, value)
}

// Get returns stored digits for the captcha id. Clear indicates
// whether the captcha must be deleted from the store.
func (s *MyCaptchaStore) Get(id string, clear bool) string {
	return s.store.Get(id, clear)
}

// Verify captcha's answer directly.
func (s *MyCaptchaStore) Verify(id, answer string, clear bool) bool {
	v := s.Get(id, clear)

	return strings.EqualFold(v, answer)
}
