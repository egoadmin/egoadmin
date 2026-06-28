package logincrypto

import "strings"

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

func (b keyBuilder) challenge(id string) string {
	return b.key("logincrypto", "challenge", id)
}
