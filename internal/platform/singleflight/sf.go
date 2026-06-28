package singleflight

import "golang.org/x/sync/singleflight"

// New 实例化
func New() *singleflight.Group {
	return &singleflight.Group{}
}
