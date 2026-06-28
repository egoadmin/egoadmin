package redis

type Option func(*Allocator)

func WithKeyPrefix(prefix string) Option {
	return func(a *Allocator) {
		if prefix != "" {
			a.keyPrefix = prefix
		}
	}
}
