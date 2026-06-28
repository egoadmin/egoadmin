package logincrypto

import "errors"

var (
	ErrInvalidConfig    = errors.New("logincrypto: invalid config")
	ErrChallengeInvalid = errors.New("logincrypto: challenge is invalid")
	ErrCipherInvalid    = errors.New("logincrypto: cipher is invalid")
	ErrKeyNotFound      = errors.New("logincrypto: key not found")
)
