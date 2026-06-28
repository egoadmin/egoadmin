package idgen

import "context"

// IDGetter adapts a named idgen generator to the legacy xflake.Geter shape.
type IDGetter struct {
	generator Generator
}

func NewIDGetter(generator Generator) *IDGetter {
	return &IDGetter{generator: generator}
}

func (g *IDGetter) Get() (uint64, error) {
	id, err := g.generator.Next(context.Background())
	if err != nil {
		return 0, err
	}
	if id < 0 {
		return 0, ErrOverflow
	}
	return uint64(id), nil
}
