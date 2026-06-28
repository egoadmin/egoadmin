package cdn

import (
	"crypto/hmac"
	//nolint:gosec // The image processor URL signature format requires HMAC-SHA1 compatibility.
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	queryExpires = "expires"
	queryToken   = "token"
)

func signAccess(secret string, material string, expires int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(material))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(strconv.FormatInt(expires, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyAccessSignature(secret string, material string, expiresRaw string, token string, now time.Time) error {
	expiresRaw = strings.TrimSpace(expiresRaw)
	token = strings.TrimSpace(token)
	if expiresRaw == "" && token == "" {
		return ErrSignatureRequired
	}
	if expiresRaw == "" || token == "" {
		return ErrSignatureInvalid
	}
	expires, err := strconv.ParseInt(expiresRaw, 10, 64)
	if err != nil {
		return ErrSignatureInvalid
	}
	if now.Unix() > expires {
		return ErrSignatureExpired
	}
	expected := signAccess(secret, material, expires)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(token)) != 1 {
		return ErrSignatureInvalid
	}
	return nil
}

func signedURL(path string, secret string, expires time.Time) string {
	path = "/" + strings.TrimLeft(path, "/")
	expiresUnix := expires.Unix()
	token := signAccess(secret, path, expiresUnix)
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%sexpires=%d&token=%s", path, separator, expiresUnix, token)
}

func imageProcessorSignature(secret string, processorPath string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(processorPath))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}
