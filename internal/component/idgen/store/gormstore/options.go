package gormstore

import "time"

type Option func(*Store)

func WithTableName(name string) Option {
	return func(s *Store) {
		if name != "" {
			s.tableName = name
		}
	}
}

func WithNowFunc(fn func() time.Time) Option {
	return func(s *Store) {
		if fn != nil {
			s.now = fn
		}
	}
}
