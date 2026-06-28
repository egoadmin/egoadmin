package authsession

import (
	"fmt"
	"strings"
)

type keyBuilder struct {
	prefix string
}

func newKeyBuilder(prefix string) keyBuilder {
	return keyBuilder{prefix: strings.TrimSuffix(prefix, ":")}
}

func (b keyBuilder) key(parts ...string) string {
	body := strings.Join(parts, ":")
	if b.prefix == "" {
		return body
	}
	return b.prefix + ":" + body
}

func (b keyBuilder) session(sid string) string {
	return b.key("auth", "session", sid)
}

func (b keyBuilder) access(jti string) string {
	return b.key("auth", "access", jti)
}

func (b keyBuilder) refresh(refreshHash string) string {
	return b.key("auth", "refresh", refreshHash)
}

func (b keyBuilder) userSessions(uid uint64) string {
	return b.key("auth", "user_sessions", fmt.Sprintf("%d", uid))
}

func (b keyBuilder) deviceSession(uid uint64, deviceHash string) string {
	return b.key("auth", "device_session", fmt.Sprintf("%d", uid), deviceHash)
}
