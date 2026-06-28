package captcha

import (
	"github.com/mojocn/base64Captcha"
)

// ICaptcha 验证码接口
type ICaptcha interface {
	GetVerifyImgString() (id string, b64s string, err error)
	VerifyString(id, answer string) bool
}

func NewCaptcha() ICaptcha {
	return &CaptchaImpl{
		driver: &base64Captcha.DriverString{
			Height:          80,
			Width:           240,
			NoiseCount:      0,
			ShowLineOptions: 0,
			Length:          5,
			Source:          "ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz2345678",
			Fonts:           []string{"chromohv.ttf"},
		},
		store: newMemoryStore(),
	}
}

// CaptchaImpl 验证码实现
type CaptchaImpl struct {
	driver *base64Captcha.DriverString
	store  base64Captcha.Store
}

// GetVerifyImgString 获取字母数字混合验证码.
func (s *CaptchaImpl) GetVerifyImgString() (id string, b64s string, err error) {
	driver := s.driver.ConvertFonts()

	c := base64Captcha.NewCaptcha(driver, s.store)

	id, b64s, err = c.Generate()

	return
}

// VerifyString 验证输入的验证码是否正确.
func (s *CaptchaImpl) VerifyString(id, answer string) bool {
	if id == "" || answer == "" {
		return false
	}

	// store.Verify(id---图片验证码的id, s----输入的验证码, true----是否不可以再次验证)
	return s.store.Verify(id, answer, true)
}
